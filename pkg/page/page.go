package page

type Page struct {
	Offset int `query:"offset" default:"0" vd:"$>=0"`
	Limit  int `query:"limit" default:"50" vd:"$>=0"`
}
