package scanner

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/core/artwork"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/server/events"
	"github.com/navidrome/navidrome/utils/pl"
	"github.com/navidrome/navidrome/utils/singleton"
	"golang.org/x/time/rate"
)

var (
	ErrAlreadyScanning = errors.New("already scanning")
)

type Scanner interface {
	ScanAll(ctx context.Context, fullRescan bool) error
	Status(context.Context) (*StatusInfo, error)
}

type StatusInfo struct {
	Scanning    bool
	LastScan    time.Time
	Count       uint32
	FolderCount uint32
}

func GetInstance(rootCtx context.Context, ds model.DataStore, cw artwork.CacheWarmer, broker events.Broker) Scanner {
	if conf.Server.DevExternalScanner {
		return GetExternalInstance(rootCtx, ds, broker)
	}
	return GetLocalInstance(rootCtx, ds, cw, broker)
}

func GetExternalInstance(rootCtx context.Context, ds model.DataStore, broker events.Broker) Scanner {
	return singleton.GetInstance(func() *controller {
		return &controller{
			scanner: &scannerExternal{rootCtx: rootCtx},
			rootCtx: rootCtx,
			ds:      ds,
			broker:  broker,
		}
	})
}

func GetLocalInstance(rootCtx context.Context, ds model.DataStore, cw artwork.CacheWarmer, broker events.Broker) Scanner {
	return singleton.GetInstance(func() *controller {
		return &controller{
			scanner: &scannerImpl{ds: ds, cw: cw},
			rootCtx: rootCtx,
			ds:      ds,
			broker:  broker,
		}
	})
}

type scannerStatus struct {
	libID     int
	fileCount uint32
	lastPath  string
	phase     string
	err       error
}

type scanner interface {
	scanAll(ctx context.Context, fullRescan bool, progress chan<- *scannerStatus)
}

type controller struct {
	scanner
	rootCtx     context.Context
	ds          model.DataStore
	broker      events.Broker
	active      atomic.Bool
	count       atomic.Uint32
	folderCount atomic.Uint32
}

func (s *controller) Status(ctx context.Context) (*StatusInfo, error) {
	lib, err := s.ds.Library(ctx).Get(1)
	if err != nil {
		log.Error(ctx, "Error getting library", err)
		return nil, err
	}
	if s.active.Load() {
		status := &StatusInfo{
			Scanning:    true,
			LastScan:    lib.LastScanAt,
			Count:       s.count.Load(),
			FolderCount: s.folderCount.Load(),
		}
		return status, nil
	}
	count, err := s.ds.MediaFile(ctx).CountAll()
	if err != nil {
		log.Error(ctx, "Error getting media file count", err)
		return nil, err
	}
	folderCount, err := s.ds.Folder(ctx).CountAll()
	if err != nil {
		log.Error(ctx, "Error getting folder count", err)
		return nil, err
	}
	return &StatusInfo{
		Scanning:    false,
		LastScan:    lib.LastScanAt,
		Count:       uint32(count),
		FolderCount: uint32(folderCount),
	}, nil
}

func (s *controller) ScanAll(requestCtx context.Context, fullRescan bool) error {
	if !s.active.CompareAndSwap(false, true) {
		log.Debug(requestCtx, "Scanner already running, ignoring request")
		return ErrAlreadyScanning
	}
	defer s.active.Store(false)

	ctx := request.AddValues(s.rootCtx, requestCtx)
	ctx = events.BroadcastToAll(ctx)
	progress := make(chan *scannerStatus, 100)
	go func() {
		defer close(progress)
		s.scanner.scanAll(ctx, fullRescan, progress)
	}()
	return s.wait(ctx, progress)
}

func (s *controller) wait(ctx context.Context, progress <-chan *scannerStatus) error {
	limiter := rate.Sometimes{Interval: conf.Server.DevActivityPanelUpdateRate}
	s.broker.SendMessage(ctx, &events.ScanStatus{Scanning: true, Count: 0, FolderCount: 0})
	s.count.Store(0)
	s.folderCount.Store(0)
	var errs []error
	defer func() {
		s.broker.SendMessage(ctx, &events.ScanStatus{
			Scanning:    false,
			Count:       int64(s.count.Load()),
			FolderCount: int64(s.folderCount.Load()),
		})
	}()
	for p := range pl.ReadOrDone(ctx, progress) {
		if p.err != nil {
			errs = append(errs, p.err)
			continue
		}
		s.count.Add(p.fileCount)
		s.folderCount.Add(1)
		limiter.Do(func() {
			s.broker.SendMessage(ctx, &events.ScanStatus{
				Scanning:    true,
				Count:       int64(s.count.Load()),
				FolderCount: int64(s.folderCount.Load()),
			})
		})
	}
	if len(errs) != 0 {
		return errors.Join(errs...)
	}
	return nil
}
