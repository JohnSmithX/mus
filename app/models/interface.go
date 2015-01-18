package models

type IServer interface {
	//initialize function
	InitServer() error

	//json text
	JSON() ([]byte, error)

	Logs() (string, error)

	Flow() (string, error)
	//actions
	ReStart() error

	Start() error

	Stop() error

	Destroy() error

	Key() string
	//db
	Update() error
	Delete() error
}
