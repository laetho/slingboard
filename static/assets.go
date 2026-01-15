package staticfiles

import "embed"

//go:embed style.css
var FS embed.FS
