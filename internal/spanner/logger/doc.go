// Package logger adapts gorm logging to Terraform providers: it implements
// gorm's logger.Interface with the same formats and level rules as gorm's
// built-in logger, while mirroring every message to terraform-plugin-log
// (tflog) so SQL activity surfaces in Terraform's structured provider logs.
//
// The conn package installs a logger from this package on every gorm session
// it creates.
package logger
