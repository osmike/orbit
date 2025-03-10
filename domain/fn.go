package domain

type FnControl interface {
	SaveUserInfo(data map[string]string)
}

type Fn func(ctrl FnControl) error
