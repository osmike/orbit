package domain

type FnControl interface {
	SaveUserData(data map[string]interface{})
}

type Fn func(ctrl FnControl) error
