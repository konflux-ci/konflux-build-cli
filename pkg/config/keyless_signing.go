package config

import "fmt"

type KeylessSigningConfig struct {
	OIDCIssuer        string
	RekorInternalURL  string
	RekorExternalURL  string
	FulcioInternalURL string
	FulcioExternalURL string
	TufInternalURL    string
	TufExternalURL    string
}

var _ KonfluxConfigPart[*KeylessSigningConfig] = (*KeylessSigningConfig)(nil)

func (c *KeylessSigningConfig) GetOIDCIssuer() string {
	return c.OIDCIssuer
}

func (c *KeylessSigningConfig) GetRekorURL() string {
	if c.RekorInternalURL != "" {
		return c.RekorInternalURL
	}
	return c.RekorExternalURL
}

func (c *KeylessSigningConfig) GetFulcioURL() string {
	if c.FulcioInternalURL != "" {
		return c.FulcioInternalURL
	}
	return c.FulcioExternalURL
}

func (c *KeylessSigningConfig) GetTufURL() string {
	if c.TufInternalURL != "" {
		return c.TufInternalURL
	}
	return c.TufExternalURL
}

func (c *KeylessSigningConfig) IsValid() bool {
	hasOIDCIssuer := c.GetOIDCIssuer() != ""
	hasRekorURL := c.GetRekorURL() != ""
	hasFulcioURL := c.GetFulcioURL() != ""
	hasTufURL := c.GetTufURL() != ""
	return hasOIDCIssuer && hasRekorURL && hasFulcioURL && hasTufURL
}

func NewKeylessSigningConfig(rawConfig KonfluxRawConfig) *KeylessSigningConfig {
	return &KeylessSigningConfig{
		OIDCIssuer:        rawConfig.DefaultOIDCIssuer,
		RekorInternalURL:  rawConfig.RekorInternalURL,
		RekorExternalURL:  rawConfig.RekorExternalURL,
		FulcioInternalURL: rawConfig.FulcioInternalURL,
		FulcioExternalURL: rawConfig.FulcioExternalURL,
		TufInternalURL:    rawConfig.TufInternalURL,
		TufExternalURL:    rawConfig.TufExternalURL,
	}
}

func (c *KeylessSigningConfig) DeepCopy() *KeylessSigningConfig {
	return &KeylessSigningConfig{
		OIDCIssuer:        c.OIDCIssuer,
		RekorInternalURL:  c.RekorInternalURL,
		RekorExternalURL:  c.RekorExternalURL,
		FulcioInternalURL: c.FulcioInternalURL,
		FulcioExternalURL: c.FulcioExternalURL,
		TufInternalURL:    c.TufInternalURL,
		TufExternalURL:    c.TufExternalURL,
	}
}

func (c *KeylessSigningConfig) ToString() string {
	return fmt.Sprintf("%+v", c)
}
