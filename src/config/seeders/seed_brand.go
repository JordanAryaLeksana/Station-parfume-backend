package seeders

import (
	"backend/src/repository"
	"fmt"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func SeedBrand(db *gorm.DB) {
	readBrandData, err := excelize.OpenFile("C:\\Webdev\\station-parfume\\Produk_Station_Parfume.xlsx")
	if err != nil {
		fmt.Printf("error while parsing data: %v\n", err)
		return
	}
	defer func() {
		if err := readBrandData.Close(); err != nil {
			fmt.Printf("error while closing file: %v\n", err)
		}
	}()

	rows, err := readBrandData.GetRows("Brands")
	if err != nil {
		fmt.Printf("error while reading rows: %v\n", err)
		return
	}

	for i, row := range rows {

		if i < 1 {
			continue
		}
		if len(row) < 3 {
			continue
		}

		nameBrand := row[0]
		descriptionBrand := row[1]
		logo := row[2]

		if nameBrand == "" || descriptionBrand == "" || logo == "" {
			continue
		}

		brand := repository.Brand{
			Name:        nameBrand,
			Description: descriptionBrand,
			Logo:        logo,
		}

		if err := db.Where("name = ?", brand.Name).FirstOrCreate(&brand).Error; err != nil {
			fmt.Printf("failed to insert brand %s: %v\n", brand.Name, err)
		}
	}
	fmt.Println("Seeding brand selesai!")
}
