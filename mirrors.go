package lifecycle

type MirrorsType struct {
	Image   string   `toml:"image" json:"image"`
	Mirrors []string `toml:"mirrors" json:"mirrors"`
}

type RunImageType struct {
	RunImage MirrorsType `json:"runImage"`
}