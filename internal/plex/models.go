// Package plex is a small read-only client for the Plex Media Server HTTP API,
// covering only the endpoints this tool needs.
package plex

// containerResponse mirrors the "MediaContainer" envelope Plex wraps every
// response in. Both Directory (libraries) and Metadata (items) are decoded from
// the same shape so a single type serves every endpoint we call.
type containerResponse struct {
	MediaContainer struct {
		Directory []Library  `json:"Directory"`
		Metadata  []Metadata `json:"Metadata"`
	} `json:"MediaContainer"`
}

// Library is a single Plex library section (movies, shows, music, ...).
type Library struct {
	ID    string `json:"key"`
	Type  string `json:"type"` // "movie", "show" or "artist"
	Title string `json:"title"`
}

// Metadata is a media item or one of its children (season, episode). Not every
// field is populated for every endpoint; unused ones stay at their zero value.
type Metadata struct {
	RatingKey    string     `json:"ratingKey"`
	Type         string     `json:"type"` // e.g. "movie", "show", "season", "episode"
	Title        string     `json:"title"`
	Year         int        `json:"year"`
	Guid         string     `json:"guid"`  // primary GUID, e.g. plex://movie/<id>
	Index        int        `json:"index"` // season number for seasons
	ShowOrdering string     `json:"showOrdering"`
	Location     []Location `json:"Location"` // folder paths (shows/music)
	Media        []Media    `json:"Media"`    // file parts (movies/episodes)

	// Plex also returns a "Guid" array of alternate IDs (imdb/tmdb/tvdb). We
	// don't use it, but it MUST be captured here: without an exact-tag field
	// for "Guid", encoding/json's case-insensitive matching unmarshals that
	// array into the "guid" string above and fails.
	AltGUIDs []struct {
		ID string `json:"id"`
	} `json:"Guid"`
}

// Location is a folder path as Plex knows it.
type Location struct {
	Path string `json:"path"`
}

// Media groups the file parts of an item.
type Media struct {
	Part []Part `json:"Part"`
}

// Part is a single media file.
type Part struct {
	File string `json:"file"`
}

// IsNonDefaultOrdering reports whether the show uses an episode ordering other
// than the library default. When it does, the original tool writes per-season
// .plexmatch files even without the --seasonprocessing flag, to preserve order.
func (m Metadata) IsNonDefaultOrdering() bool {
	switch m.ShowOrdering {
	case "absolute", "aired", "dvd", "tmdb":
		return true
	default:
		return false
	}
}
