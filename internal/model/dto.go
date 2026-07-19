package model

// MetaDto extends Meta with an article count, mirroring the Java MetaDto.
type MetaDto struct {
	Meta
	Count int `gorm:"column:count" json:"count"`
}

// ArchiveBo groups articles by "yyyy年MM月". Mirrors ArchiveBo.
type ArchiveBo struct {
	Date     string    `json:"date"`
	Count    string    `json:"count"`
	Articles []Content `json:"articles"`
}

// CommentBo is a parent comment with its (currently flat) child list. Mirrors CommentBo.
type CommentBo struct {
	Comment
	Levels   int       `json:"levels"`
	Children []Comment `json:"children"`
}

// StatisticsBo holds the admin dashboard counters. Mirrors StatisticsBo.
type StatisticsBo struct {
	Articles int64 `json:"articles"`
	Comments int64 `json:"comments"`
	Links    int64 `json:"links"`
	Attachs  int64 `json:"attachs"`
}

// BackResponseBo holds backup paths. Mirrors BackResponseBo.
type BackResponseBo struct {
	AttachPath string `json:"attachPath"`
	ThemePath  string `json:"themePath"`
	SqlPath    string `json:"sqlPath"`
}
