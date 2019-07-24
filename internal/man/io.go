package man

type bitbucket struct{}

func (b *bitbucket) Write(p []byte) (n int, err error) {
	return len(p), nil
}
