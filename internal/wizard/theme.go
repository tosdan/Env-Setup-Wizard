package wizard

import "charm.land/huh/v2"

func envWizardTheme(isDark bool) *huh.Styles {
	styles := huh.ThemeCharm(isDark)
	accent := styles.Focused.TextInput.Prompt.GetForeground()
	styles.Focused.Base = styles.Focused.Base.BorderForeground(accent)
	styles.Focused.Card = styles.Focused.Base
	return styles
}

func applyEnvWizardTheme(form *huh.Form) *huh.Form {
	return form.WithTheme(huh.ThemeFunc(envWizardTheme))
}
