package domain

type FnControl interface {
	SaveUserInfo(data map[string]interface{})
}

type Fn func(ctrl FnControl) error
