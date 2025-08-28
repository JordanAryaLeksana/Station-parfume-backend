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
	fmt.Printf("Found %d rows\n", len(rows))

	for i, row := range rows {

		if i < 1 {
			fmt.Printf("Skipping header row %d\n", i+1)
			continue
		}

		nameBrand := row[0]

		// logo := row[2]
		// if logo == "" {
		// 	logo = "default-logo.png"
		// }

		// if nameBrand == "" {
		// 	continue
		// }

		// descriptionBrand := row[1]
		// if descriptionBrand == "" {
		// 	descriptionBrand = "No description yet"
		// }

		brand := repository.Brand{
			Name:        nameBrand,
			Description: "No description yet",
			Logo:        "default-logo.png",
		}

		if err := db.Create(&brand).Error; err != nil {
			fmt.Printf("gagal insert %s: %v\n", nameBrand, err)
		} else {
			fmt.Printf("sukses insert: %s\n", nameBrand)
		}
	}
	fmt.Println("Seeding brand selesai!")
}
