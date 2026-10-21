package species

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

// TestLive calls the real providers once each (about 5 calls in all). It is skipped
// unless GROWER_LIVE=1, and it never prints credentials, only mapped fields:
//
//	set -a; . ~/.secrets/grower.env; set +a; GROWER_LIVE=1 go test ./internal/species -run TestLive -v
func TestLive(t *testing.T) {
	if os.Getenv("GROWER_LIVE") != "1" {
		t.Skip("set GROWER_LIVE=1 and the provider variables to call the real APIs")
	}
	var providers []Provider
	if id, secret := os.Getenv("GROWER_OPENPLANTBOOK_CLIENT_ID"), os.Getenv("GROWER_OPENPLANTBOOK_CLIENT_SECRET"); id != "" && secret != "" {
		providers = append(providers, NewOpenPlantbook(id, secret))
	}
	if key := os.Getenv("GROWER_PERENUAL_API_KEY"); key != "" {
		providers = append(providers, NewPerenual(key))
	}
	for _, p := range providers {
		t.Run(p.Source(), func(t *testing.T) {
			ctx := context.Background()
			q := cmp.Or(os.Getenv("GROWER_LIVE_QUERY"), "monstera")
			found, err := p.Search(ctx, q)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			t.Logf("search: %d candidates", len(found))
			if len(found) == 0 {
				return
			}
			ref := found[0].Ref
			for _, c := range found {
				if c.ScientificName == "Monstera deliciosa" || c.Ref == "monstera deliciosa" {
					ref = c.Ref
					break
				}
			}
			t.Logf("first: %+v; importing %q", found[0], ref)
			care, err := p.Details(ctx, ref)
			if err != nil {
				t.Fatalf("details: %v", err)
			}
			t.Logf("care: name=%q common=%q interval=%s light=%s water_at=%s",
				care.ScientificName, care.CommonName, text(care.WaterIntervalDays), text(care.Light), text(care.WaterAt))
			var fields map[string]json.RawMessage
			_ = json.Unmarshal(care.Raw, &fields)
			for _, k := range []string{"watering", "watering_general_benchmark", "sunlight", "min_light_lux", "max_light_lux", "min_soil_moist", "max_soil_moist", "alias"} {
				if v, ok := fields[k]; ok {
					t.Logf("raw %s = %s", k, v)
				}
			}
		})
	}
}

// TestLivePlantNet identifies one real photo (one call). It is skipped unless
// GROWER_LIVE=1, GROWER_PLANTNET_API_KEY and GROWER_LIVE_IMAGE (a JPEG or PNG of a real
// plant) are set, and it prints only the mapped candidates:
//
//	set -a; . ~/.secrets/grower.env; set +a; GROWER_LIVE=1 GROWER_LIVE_IMAGE=leaf.jpg go test ./internal/species -run TestLivePlantNet -v
func TestLivePlantNet(t *testing.T) {
	key, path := os.Getenv("GROWER_PLANTNET_API_KEY"), os.Getenv("GROWER_LIVE_IMAGE")
	if os.Getenv("GROWER_LIVE") != "1" || key == "" || path == "" {
		t.Skip("set GROWER_LIVE=1, GROWER_PLANTNET_API_KEY and GROWER_LIVE_IMAGE to call Pl@ntNet")
	}
	image, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	found, err := NewPlantNet(key).Identify(context.Background(), image, http.DetectContentType(image), cmp.Or(os.Getenv("GROWER_LIVE_ORGAN"), "leaf"))
	if err != nil {
		t.Fatalf("identify: %v", redactKey(err, key))
	}
	for _, f := range found {
		t.Logf("candidate: %q (%q) score %.3f", f.ScientificName, f.CommonName, f.Score)
	}
}
