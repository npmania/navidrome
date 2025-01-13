import React from 'react'
import PropTypes from 'prop-types'
import { Link } from 'react-admin'
import { withWidth } from '@material-ui/core'
import { useGetHandleArtistClick } from './useGetHandleArtistClick'
import { intersperse } from '../utils/index.js'

const ALink = withWidth()((props) => {
  const { artist, width, ...rest } = props
  const artistLink = useGetHandleArtistClick(width)
  return (
    <Link
      key={artist.id}
      to={artistLink(artist.id)}
      onClick={(e) => e.stopPropagation()}
      {...rest}
    >
      {artist.name}
    </Link>
  )
})

const parseAndReplaceArtists = (
  displayAlbumArtist,
  albumArtists,
  className,
) => {
  let result = []
  let lastIndex = 0

  albumArtists.forEach((artist) => {
    const index = displayAlbumArtist.indexOf(artist.name, lastIndex)
    if (index !== -1) {
      // Add text before the artist name
      if (index > lastIndex) {
        result.push(displayAlbumArtist.slice(lastIndex, index))
      }
      // Add the artist link
      result.push(<ALink artist={artist} className={className}></ALink>)
      lastIndex = index + artist.name.length
    }
  })

  // Add any remaining text after the last artist name
  if (lastIndex < displayAlbumArtist.length) {
    result.push(displayAlbumArtist.slice(lastIndex))
  }

  return result
}

export const ArtistLinkField = ({ record, className, source }) => {
  const role = source.toLowerCase()
  const artists = record['participants']
    ? record['participants'][role]
    : [{ name: record[source], id: record[source + 'Id'] }]

  // When showing artists for a track, add any remixers to the list of artists
  if (
    role === 'artist' &&
    record['participants'] &&
    record['participants']['remixer']
  ) {
    record['participants']['remixer'].forEach((remixer) => {
      artists.push(remixer)
    })
  }

  if (role === 'albumartist') {
    return (
      <div className={className}>
        {parseAndReplaceArtists(record[source], artists, className)}
      </div>
    )
  }

  // Dedupe artists, only shows the first 3
  const seen = new Set()
  const dedupedArtists = []
  artists?.forEach((artist) => {
    if (!seen.has(artist.id) && dedupedArtists.length < 3) {
      seen.add(artist.id)
      dedupedArtists.push(artist)
    }
  })

  const artistsList = dedupedArtists.map((artist) => (
    <ALink artist={artist} className={className} key={artist?.id} />
  ))

  return <>{intersperse(artistsList, ' • ')}</>
}

ArtistLinkField.propTypes = {
  record: PropTypes.object,
  className: PropTypes.string,
  source: PropTypes.string,
}

ArtistLinkField.defaultProps = {
  addLabel: true,
  source: 'albumArtist',
}
