// Package locales embeds the default translations independently of gameplay content.
package locales

import "embed"

//go:embed en.json zh-CN.json
var Files embed.FS
