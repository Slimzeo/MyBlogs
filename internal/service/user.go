package service

import (
	"strconv"
	"strings"

	"myblog/internal/model"
	"myblog/internal/util"

	"golang.org/x/crypto/bcrypt"
)

// InsertUser saves a new user with bcrypt.
// Mirrors UserServiceImpl.insertUser.
func (s *Service) InsertUser(u *model.User) (int, error) {
	if strings.TrimSpace(u.Username) != "" && strings.TrimSpace(u.Email) != "" {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return 0, err
		}
		u.Password = string(passwordHash)
		if err := s.db.Create(u).Error; err != nil {
			return 0, err
		}
	}
	return u.Uid, nil
}

// QueryUserByID fetches a user by id. Mirrors queryUserById.
func (s *Service) QueryUserByID(uid int) *model.User {
	if uid == 0 {
		return nil
	}
	key := "user:" + strconv.Itoa(uid)
	if cached, exists := s.cache.Get(key); exists {
		user := cached.(model.User)
		return &user
	}
	var u model.User
	if err := s.db.First(&u, uid).Error; err != nil {
		return nil
	}
	s.cache.Set(key, u, 30)
	return &u
}

// Login accepts bcrypt and the legacy Java MD5(username+password) format. A
// successful legacy login transparently upgrades the stored hash to bcrypt.
func (s *Service) Login(username, password string) (*model.User, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return nil, Tip("用户名和密码不能为空")
	}
	var count int64
	s.db.Model(&model.User{}).Where("username = ?", username).Count(&count)
	if count < 1 {
		return nil, Tip("用户名或密码错误")
	}
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, Tip("用户名或密码错误")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) == nil {
		return &user, nil
	}
	if user.Password != util.MD5encode(username+password) {
		return nil, Tip("用户名或密码错误")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err == nil {
		user.Password = string(passwordHash)
		_ = s.db.Model(&model.User{}).Where("uid = ?", user.Uid).
			Update("password", user.Password).Error
		s.cache.Del("user:" + strconv.Itoa(user.Uid))
	}
	return &user, nil
}

// UpdateUserByUID selectively updates a user. Mirrors updateByUid.
func (s *Service) UpdateUserByUID(u *model.User) error {
	if u == nil || u.Uid == 0 {
		return Tip("userVo is null")
	}
	updates := map[string]interface{}{}
	if u.Username != "" {
		if len([]rune(u.Username)) > 32 {
			return Tip("用户名不能超过32个字符")
		}
		var count int64
		if err := s.db.Model(&model.User{}).
			Where("username = ? AND uid <> ?", u.Username, u.Uid).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return Tip("用户名已存在")
		}
		updates["username"] = u.Username
	}
	if u.Password != "" {
		updates["password"] = u.Password
	}
	if u.Email != "" {
		var count int64
		if err := s.db.Model(&model.User{}).
			Where("email = ? AND uid <> ?", u.Email, u.Uid).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return Tip("邮箱已存在")
		}
		updates["email"] = u.Email
	}
	if u.ScreenName != "" {
		updates["screen_name"] = u.ScreenName
	}
	if u.HomeURL != "" {
		updates["home_url"] = u.HomeURL
	}
	if len(updates) == 0 {
		return nil
	}
	res := s.db.Model(&model.User{}).Where("uid = ?", u.Uid).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	s.cache.Del("user:" + strconv.Itoa(u.Uid))
	return nil
}
