package model

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/navidrome/navidrome/consts"
)

type Tag struct {
	ID    string
	Name  string
	Value string
}

type FlattenedTags []Tag

func (t Tag) String() string {
	return fmt.Sprintf("%s=%s", t.Name, t.Value)
}

type Tags map[string][]string

func (t Tags) Values(name string) []string {
	return t[name]
}

func (t Tags) Flatten(name string) FlattenedTags {
	var tags FlattenedTags
	for _, v := range t[name] {
		tags = append(tags, NewTag(name, v))
	}
	return tags
}

func (t Tags) FlattenAll() FlattenedTags {
	var tags FlattenedTags
	for name, values := range t {
		for _, v := range values {
			tags = append(tags, NewTag(name, v))
		}
	}
	return tags
}

func NewTag(name, value string) Tag {
	name = strings.ToLower(name)
	id := fmt.Sprintf("%x", md5.Sum([]byte(name+consts.Zwsp+strings.ToLower(value))))
	return Tag{
		ID:    id,
		Name:  name,
		Value: value,
	}
}

type TagRepository interface {
	Add(...Tag) error
}