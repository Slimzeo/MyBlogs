package service

import "myblog/internal/model"

// GetOptions returns all site options. Mirrors OptionServiceImpl.getOptions.
func (s *Service) GetOptions() []model.Option {
	var list []model.Option
	s.db.Find(&list)
	return list
}

// OptionsMap returns options as a name->value map (used to hydrate the config cache).
func (s *Service) OptionsMap() map[string]string {
	m := map[string]string{}
	for _, o := range s.GetOptions() {
		m[o.Name] = o.Value
	}
	return m
}

// SaveOption upserts a single option. Mirrors insertOption(name,value) which
// updated when the row already existed.
func (s *Service) SaveOption(name, value string) error {
	var count int64
	s.db.Model(&model.Option{}).Where("name = ?", name).Count(&count)
	if count == 0 {
		return s.db.Create(&model.Option{Name: name, Value: value}).Error
	}
	return s.db.Model(&model.Option{}).Where("name = ?", name).Update("value", value).Error
}

// SaveOptions upserts a batch of options. Mirrors saveOptions.
func (s *Service) SaveOptions(options map[string]string) error {
	for k, v := range options {
		if err := s.SaveOption(k, v); err != nil {
			return err
		}
	}
	return nil
}
