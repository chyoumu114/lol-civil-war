package champion

import _ "embed"

// embeddedLibrary 是随二进制发布的英雄资料库（约 173 名英雄）。
//
//go:embed library.json
var embeddedLibrary []byte
