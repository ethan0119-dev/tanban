package app

import "testing"

func TestOptionalWebsiteURLAcceptsBundledAssets(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value string
		want  bool
	}{
		{"", true},
		{"/website/scan-ordering.png", true},
		{"https://api.tanban.com.cn/api/v1/public/media/website/example.png", true},
		{"/website/../secret.png", false},
		{"/other/image.png", false},
		{"javascript:alert(1)", false},
	}
	for _, test := range tests {
		if got := validOptionalWebsiteURL(test.value); got != test.want {
			t.Fatalf("validOptionalWebsiteURL(%q)=%v, want %v", test.value, got, test.want)
		}
	}
}
