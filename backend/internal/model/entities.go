package model

import "time"

type User struct {
	ID        int64
	Email     string
	Name      string
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type APIKey struct {
	ID        int64
	UserID    int64
	KeyHash   string
	Label     string
	CreatedAt time.Time
	RevokedAt *time.Time
}

type Tier struct {
	ID           int64
	Code         string
	Name         string
	PriceMicros  int64
	ValueType    string
	ValueAmount  int64
	Description  string
	FeaturesJSON string
	IsEnabled    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ServiceItem struct {
	ID        int64
	Code      string
	Name      string
	Unit      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TierDefaultItem struct {
	ID            int64
	TierID        int64
	ServiceItemID int64
	IncludedUnits int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Subscription struct {
	ID        int64
	UserID    int64
	TierID    int64
	Status    string
	StartedAt time.Time
	EndedAt   *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SubscriptionOverride struct {
	ID             int64
	SubscriptionID int64
	ServiceItemID  int64
	IncludedUnits  int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UnitPrice struct {
	ID                 int64
	ServiceItemID      int64
	TierID             *int64
	PricePerUnitMicros int64
	Currency           string
	EffectiveFrom      time.Time
	EffectiveTo        *time.Time
	CreatedAt          time.Time
}

type UsageRecord struct {
	ID            int64
	UserID        int64
	APIKeyID      *int64
	ServiceItemID int64
	Quantity      int64
	UsageAt       time.Time
	MetadataJSON  *string
	CreatedAt     time.Time
}

type SoftwareConfig struct {
	ID           int64
	SoftwareCode string
	SoftwareName string
	GroupID      int64
	Description  string
	IsEnabled    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SoftwareTag struct {
	ID               int64
	SoftwareConfigID int64
	Tag              string
	CreatedAt        time.Time
}

type ConfigTemplate struct {
	ID               int64
	SoftwareConfigID int64
	Name             string
	Format           string
	Content          string
	IsDefault        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type GlobalTemplateVar struct {
	ID          int64
	VarKey      string
	VarValue    string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserSyncedConfig struct {
	ID        int64
	UserID    int64
	UUID      string
	Software  string
	Name      string
	FilePath  string
	Version   string
	InUse     bool
	Selected  bool
	Format    string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Download struct {
	ID           int64
	SoftwareName string
	Platform     string
	FileType     string
	DownloadURL  string
	Version      string
	ForceUpdate  bool
	Changelog    string
	IsDefault    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DocCategory struct {
	ID          int64
	Slug        string
	Title       string
	Description *string
	Icon        *string
	SortOrder   int64
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DocPage struct {
	ID         int64
	Slug       string
	Title      string
	CategoryID int64
	MDXBody    string
	SortOrder  int64
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Article struct {
	ID              int64
	LegacyID        *int64
	Slug            string
	Title           string
	Excerpt         *string
	CoverImageURL   *string
	Tag             *string
	ReadTime        *string
	AuthorName      *string
	AuthorAvatarURL *string
	AuthorIcon      *string
	MDXBody         string
	Status          string
	PublishedAt     *time.Time
	CreatedByUserID *int64
	UpdatedByUserID *int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
