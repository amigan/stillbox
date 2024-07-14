package config

type Config struct {
	DB        string `yaml:"db"`
	JWTSecret string `yaml:"jwtsecret"`
	Listen    string `yaml:"listen"`
	Public    bool   `yaml:"public"`
}

func ReadConfig() (*Config, error) {
	return &Config{
		JWTSecret: "s3c4r",
		Listen: ":3050",
	}, nil
}
