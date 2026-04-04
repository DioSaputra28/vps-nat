package telegram

import (
	"github.com/DioSaputra28/vps-nat/internal/model"
)

func packageQuote(pkg *model.Package) PackageQuote {
	return PackageQuote{
		ID:           pkg.ID,
		Name:         pkg.Name,
		CPU:          pkg.CPU,
		RAMMB:        pkg.RAMMB,
		DiskGB:       pkg.DiskGB,
		Price:        pkg.Price,
		DurationDays: pkg.DurationDays,
	}
}

func defaultImageOptions() []ImageOption {
	return []ImageOption{
		{Label: "Debian 12", Alias: "images:debian/12"},
		{Label: "Debian 13", Alias: "images:debian/13"},
		{Label: "Ubuntu 22.04", Alias: "images:ubuntu/22.04"},
		{Label: "Ubuntu 24.04", Alias: "images:ubuntu/24.04"},
		{Label: "Kali Linux", Alias: "images:kali/current"},
	}
}

func findImageOption(alias string) (ImageOption, bool) {
	for _, image := range defaultImageOptions() {
		if image.Alias == alias {
			return image, true
		}
	}
	return ImageOption{}, false
}
