package model

// These structs mirror the original MyBatis Vo classes and the tale.sql schema.
// Column names/tables are kept identical to the Java project so an existing
// MySQL `tale` database works unchanged.

// Content is the t_contents table (articles and pages). Maps ContentVo.
type Content struct {
	Cid          int    `gorm:"column:cid;primaryKey;autoIncrement" json:"cid"`
	Title        string `gorm:"column:title" json:"title"`
	Slug         string `gorm:"column:slug;uniqueIndex;default:null" json:"slug"`
	Created      int    `gorm:"column:created;index" json:"created"`
	Modified     int    `gorm:"column:modified" json:"modified"`
	Content      string `gorm:"column:content;type:text" json:"content"`
	AuthorID     int    `gorm:"column:author_id" json:"authorId"`
	Type         string `gorm:"column:type" json:"type"`
	Status       string `gorm:"column:status" json:"status"`
	Tags         string `gorm:"column:tags" json:"tags"`
	Categories   string `gorm:"column:categories" json:"categories"`
	Hits         int    `gorm:"column:hits" json:"hits"`
	CommentsNum  int    `gorm:"column:comments_num" json:"commentsNum"`
	AllowComment bool   `gorm:"column:allow_comment" json:"allowComment"`
	AllowPing    bool   `gorm:"column:allow_ping" json:"allowPing"`
	AllowFeed    bool   `gorm:"column:allow_feed" json:"allowFeed"`
}

func (Content) TableName() string { return "t_contents" }

// Comment is the t_comments table. Maps CommentVo.
type Comment struct {
	Coid     int    `gorm:"column:coid;primaryKey;autoIncrement" json:"coid"`
	Cid      int    `gorm:"column:cid;index" json:"cid"`
	Created  int    `gorm:"column:created;index" json:"created"`
	Author   string `gorm:"column:author" json:"author"`
	AuthorID int    `gorm:"column:author_id" json:"authorId"`
	OwnerID  int    `gorm:"column:owner_id" json:"ownerId"`
	Mail     string `gorm:"column:mail" json:"mail"`
	URL      string `gorm:"column:url" json:"url"`
	IP       string `gorm:"column:ip" json:"ip"`
	Agent    string `gorm:"column:agent" json:"agent"`
	Content  string `gorm:"column:content;type:text" json:"content"`
	Type     string `gorm:"column:type" json:"type"`
	Status   string `gorm:"column:status" json:"status"`
	Parent   int    `gorm:"column:parent" json:"parent"`
}

func (Comment) TableName() string { return "t_comments" }

// Meta is the t_metas table (categories, tags, links). Maps MetaVo.
type Meta struct {
	Mid         int    `gorm:"column:mid;primaryKey;autoIncrement" json:"mid"`
	Name        string `gorm:"column:name;uniqueIndex:idx_meta_type_name" json:"name"`
	Slug        string `gorm:"column:slug;index" json:"slug"`
	Type        string `gorm:"column:type;uniqueIndex:idx_meta_type_name" json:"type"`
	Description string `gorm:"column:description" json:"description"`
	Sort        int    `gorm:"column:sort" json:"sort"`
	Parent      int    `gorm:"column:parent" json:"parent"`
}

func (Meta) TableName() string { return "t_metas" }

// User is the t_users table. Maps UserVo.
type User struct {
	Uid        int    `gorm:"column:uid;primaryKey;autoIncrement" json:"uid"`
	Username   string `gorm:"column:username;uniqueIndex;size:32" json:"username"`
	Password   string `gorm:"column:password" json:"-"`
	Email      string `gorm:"column:email;uniqueIndex;size:200" json:"email"`
	HomeURL    string `gorm:"column:home_url" json:"homeUrl"`
	ScreenName string `gorm:"column:screen_name" json:"screenName"`
	Created    int    `gorm:"column:created" json:"created"`
	Activated  int    `gorm:"column:activated" json:"activated"`
	Logged     int    `gorm:"column:logged" json:"logged"`
	GroupName  string `gorm:"column:group_name" json:"groupName"`
}

func (User) TableName() string { return "t_users" }

// Option is the t_options table (key/value site settings). Maps OptionVo.
type Option struct {
	Name        string `gorm:"column:name;primaryKey;size:32" json:"name"`
	Value       string `gorm:"column:value" json:"value"`
	Description string `gorm:"column:description" json:"description"`
}

func (Option) TableName() string { return "t_options" }

// Relationship is the t_relationships join table (content <-> meta). Maps RelationshipVoKey.
type Relationship struct {
	Cid int `gorm:"column:cid;primaryKey" json:"cid"`
	Mid int `gorm:"column:mid;primaryKey" json:"mid"`
}

func (Relationship) TableName() string { return "t_relationships" }

// Attach is the t_attach table (uploaded files). Maps AttachVo.
type Attach struct {
	ID       int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Fname    string `gorm:"column:fname" json:"fname"`
	Ftype    string `gorm:"column:ftype" json:"ftype"`
	Fkey     string `gorm:"column:fkey" json:"fkey"`
	AuthorID int    `gorm:"column:author_id" json:"authorId"`
	Created  int    `gorm:"column:created" json:"created"`
}

func (Attach) TableName() string { return "t_attach" }

// Log is the t_logs table (admin operation log). Maps LogVo.
type Log struct {
	ID       int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Action   string `gorm:"column:action" json:"action"`
	Data     string `gorm:"column:data" json:"data"`
	AuthorID int    `gorm:"column:author_id" json:"authorId"`
	IP       string `gorm:"column:ip" json:"ip"`
	Created  int    `gorm:"column:created" json:"created"`
}

func (Log) TableName() string { return "t_logs" }
