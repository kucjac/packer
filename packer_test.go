// +build integration

package packer

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"image"
	"image/jpeg"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPacker tests the image packer
func TestPacker(t *testing.T) {
	t.Run("Packing", func(t *testing.T) {
		p := New(DefaultConfig())

		var files []string
		err := filepath.Walk("./tests", func(path string, info os.FileInfo, err error) error {
			if strings.HasSuffix(path, "jpg") && !strings.HasPrefix(info.Name(), "joined") {
				files = append(files, path)
			}
			return nil
		})
		require.NoError(t, err)

		for _, file := range files {
			t.Logf("Reading: %s", file)
			f, err := os.Open(file)
			require.NoError(t, err)
			defer f.Close()
			_, err = p.AddImageReader(f)
			require.NoError(t, err)
		}

		require.NoError(t, p.Pack())

		for i, img := range p.OutputImages {
			t.Logf("Writing image: %d", i)
			f, err := os.Create(filepath.Join("tests", fmt.Sprintf("joined_%d.jpg", i)))
			require.NoError(t, err)
			defer f.Close()
			require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 100}))
		}

	})

	t.Run("Consistency", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AutoGrow = true
		cfg.Square = false
		p := New(cfg)

		var files []string
		err := filepath.Walk("./tests", func(path string, info os.FileInfo, err error) error {
			if strings.HasSuffix(path, "jpg") && !strings.HasPrefix(info.Name(), "joined") && !strings.HasPrefix(info.Name(), "consist") {
				files = append(files, path)
			}
			return nil
		})
		require.NoError(t, err)

		// for i := 0; i < 15; i++ {
		for _, file := range files {
			t.Logf("Reading: %s", file)
			f, err := os.Open(file)
			require.NoError(t, err)
			defer f.Close()
			_, err = p.AddImageReader(f)
			require.NoError(t, err)
		}

		require.NoError(t, p.Pack())

		output := p.OutputImages[0]
		require.NotNil(t, output)
		for _, img := range p.OutputImages {
			if testing.Verbose() {
				f, err := os.Create(filepath.Join("tests", "consist.jpg"))
				require.NoError(t, err)
				defer f.Close()
				require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 100}))
			}
		}

		for _, img := range p.images.inputImages {

			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					c1 := img.image.At(x, y)
					c2 := output.At(img.pos.X+x, img.pos.Y+y)
					assert.Equal(t, c1, c2)
				}
			}
			if testing.Verbose() {
				f, err := os.Create(filepath.Join("tests", fmt.Sprintf("consist_sub_%d_%d.jpg", img.pos.X, img.pos.Y)))
				require.NoError(t, err)
				defer f.Close()
				require.NoError(t, jpeg.Encode(f, img.image, &jpeg.Options{Quality: 100}))
			}
		}
	})

	t.Run("Dups", func(t *testing.T) {
		cfg1 := DefaultConfig()
		cfg1.TextureHeight = 4 * 8196
		cfg1.TextureWidth = 4 * 8196

		cfg2 := DefaultConfig()
		cfg2.AutoGrow = true
		// cfg.AutoGrow = false
		// cfg.Square = false

		type configName struct {
			c    *Config
			name string
		}
		var configs = []*configName{
			{c: cfg1, name: "Normal"},
			{c: cfg2, name: "AutoGrow"},
		}

		testFunc := func(t *testing.T, cfg *Config) {
			p := New(cfg)

			var files []string
			err := filepath.Walk("./rl_tests", func(path string, info os.FileInfo, err error) error {
				if strings.HasSuffix(path, "jpg") && !strings.HasPrefix(info.Name(), "joined") {
					files = append(files, path)
				}
				return nil
			})
			require.NoError(t, err)

			var images []*InputImage
			for _, file := range files {
				f, err := os.Open(file)
				require.NoError(t, err)

				i, err := p.AddImageReader(f)
				require.NoError(t, err)
				images = append(images, i)
			}

			require.NoError(t, p.Pack())

			var poses = map[string]struct{}{}
			for _, i := range images {
				if i.pos.Eq(image.Pt(999999, 999999)) {
					continue
				}
				_, ok := poses[i.pos.String()]
				require.False(t, ok, i.pos.String())

				poses[i.pos.String()] = struct{}{}

			}
		}

		for _, c := range configs {
			t.Run(c.name, func(t *testing.T) {
				testFunc(t, c.c)
			})
		}

	})
}

// BenchmarkPacker banches the packer
func BenchmarkPacker(b *testing.B) {

	var images [][]byte

	var files []string
	err := filepath.Walk("./tests", func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, "jpg") && !strings.HasPrefix(info.Name(), "joined") {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(b, err)

	for _, file := range files {
		f, err := os.Open(file)
		require.NoError(b, err)
		defer f.Close()

		data, err := ioutil.ReadAll(f)
		require.NoError(b, err)

		images = append(images, data)
	}

	getPacker := func(b *testing.B) *Packer {
		p := New(DefaultConfig())
		for _, image := range images {
			_, err := p.AddImageBytes(image)
			require.NoError(b, err)
		}
		return p
	}

	b.Run("Growing", func(b *testing.B) {
		b.Run("Square", func(b *testing.B) {
			b.Run("Default", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					p := getPacker(b)

					p.cfg.AutoGrow = true

					b.ResetTimer()
					b.StartTimer()
					require.NoError(b, p.Pack())
					b.StopTimer()
				}
			})

			b.Run("Double", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					p := getPacker(b)
					p.cfg.AutoGrow = true
					p.cfg.TextureHeight *= 2
					p.cfg.TextureWidth *= 2

					b.ResetTimer()
					b.StartTimer()
					require.NoError(b, p.Pack())
					b.StopTimer()
				}
			})

			b.Run("Quad", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					p := getPacker(b)
					p.cfg.AutoGrow = true
					p.cfg.TextureHeight *= 4
					p.cfg.TextureWidth *= 4

					b.ResetTimer()
					b.StartTimer()
					require.NoError(b, p.Pack())
					b.StopTimer()
				}
			})

		})

		b.Run("NonSquare", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				p := getPacker(b)
				p.cfg.Square = false
				p.cfg.AutoGrow = true

				b.StartTimer()
				require.NoError(b, p.Pack())
				b.StopTimer()
			}
		})

	})

	b.Run("1024x1024", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p := getPacker(b)
			p.cfg.TextureHeight = 1024
			p.cfg.TextureWidth = 1024
			p.cfg.AutoGrow = false

			b.StartTimer()
			require.NoError(b, p.Pack())
			b.StopTimer()
		}
	})

	b.Run("4096x4096", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p := getPacker(b)
			p.cfg.TextureHeight = 4096
			p.cfg.TextureWidth = 4096
			p.cfg.AutoGrow = false

			b.StartTimer()
			require.NoError(b, p.Pack())
			b.StopTimer()
		}
	})

	// b.Run("Pack", func(b *testing.B) {
	// 	p := getPacker(b)
	// 	// p.
	// })

	b.Run("Pack", func(b *testing.B) {

		b.Run("pack", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				p := getPacker(b)
				b.StartTimer()

				require.NoError(b, p.pack(p.cfg.Heuristic, p.cfg.TextureWidth, p.cfg.TextureHeight))
			}
		})

		b.Run("createBinImages", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				p := getPacker(b)

				p.pack(p.cfg.Heuristic, p.cfg.TextureWidth, p.cfg.TextureHeight)

				require.NoError(b, p.createBinImages())

			}
		})
	})
}
