package domain

import "testing"

func TestParseYouTubeID(t *testing.T) {
	t.Parallel()
	id := "dQw4w9WgXcQ"
	cases := []string{
		id,
		"https://www.youtube.com/watch?v=" + id + "&t=10",
		"https://youtu.be/" + id,
		"https://youtube.com/shorts/" + id,
		"https://youtube.com/live/" + id,
	}
	for _, value := range cases {
		got, err := ParseYouTubeID(value)
		if err != nil || got != id {
			t.Fatalf("ParseYouTubeID(%q) = %q, %v", value, got, err)
		}
	}
}

func TestParseYouTubeIDRejectsOtherHosts(t *testing.T) {
	t.Parallel()
	if _, err := ParseYouTubeID("https://example.com/watch?v=dQw4w9WgXcQ"); err == nil {
		t.Fatal("expected unsupported host error")
	}
}

func TestRBAC(t *testing.T) {
	t.Parallel()
	if Can(RoleUser, "moderate") {
		t.Fatal("user must not moderate")
	}
	if !Can(RoleModerator, "moderate") {
		t.Fatal("moderator must moderate")
	}
	if Can(RoleModerator, "manage") {
		t.Fatal("moderator must not manage")
	}
	if !Can(RoleOwner, "manage") {
		t.Fatal("owner must manage")
	}
}
