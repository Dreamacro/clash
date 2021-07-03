package obfs

func init() {
	register("http_post", newHTTPPost, 0)
}

func newHTTPPost(b *Base) Obfs {
	return &httpObfs{Base: b, post: true}
}
