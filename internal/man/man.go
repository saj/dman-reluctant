package man

// Key identifies a Debian manual page document.
type Key struct {
	Page string
	Dist string
	Lang string
}
