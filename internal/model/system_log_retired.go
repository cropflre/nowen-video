package model

// SystemLog is a temporary compile-time alias used only by the historical
// AutoMigrate list in model.go. It no longer represents a log entity or a
// system_logs table; the alias resolves to SystemSetting, which is already
// migrated normally. The persisted system logging feature has been removed.
type SystemLog = SystemSetting
