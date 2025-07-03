package config

import "github.com/spf13/viper"

type Profile struct{}

func (p *Profile) WriteConfigField(field, value string) error {
	viper.ReadInConfig()
	viper.Set(p.GetConfigField(field), value)
	return viper.WriteConfig()
}

// GetConfigField returns the configuration field for the specific profile
func (p *Profile) GetConfigField(field string) string {
	// return p.ProfileName + "." + field
	return ""
}

// DeleteConfigField deletes a configuration field.
func (p *Profile) DeleteConfigField(field string) error {
	return nil
}
