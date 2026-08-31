package wizard

import (
	"reflect"
	"testing"
)

func TestEnvWizardThemeUsesPromptAccentForFocusedBorder(t *testing.T) {
	for _, isDark := range []bool{false, true} {
		styles := envWizardTheme(isDark)
		promptColor := styles.Focused.TextInput.Prompt.GetForeground()
		borderColor := styles.Focused.Base.GetBorderLeftForeground()

		if !reflect.DeepEqual(borderColor, promptColor) {
			t.Errorf(
				"envWizardTheme(%t) border color = %v, want prompt color %v",
				isDark,
				borderColor,
				promptColor,
			)
		}
		if !reflect.DeepEqual(styles.Focused.Card, styles.Focused.Base) {
			t.Errorf("envWizardTheme(%t) focused card does not match focused base", isDark)
		}
	}
}
