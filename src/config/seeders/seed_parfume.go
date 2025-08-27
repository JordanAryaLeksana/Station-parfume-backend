package seeders

import (
	"backend/src/repository"
	"fmt"
	"log"
	"strconv"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func SeedParfume(db *gorm.DB) {
	readParfumeData, err := excelize.OpenFile("C:\\Webdev\\station-parfume\\Produk_Station_Parfume.xlsx")
	if err != nil {
		log.Fatalf("error while parsing data: %v", err)
	}
	defer func() {
		if err := readParfumeData.Close(); err != nil {
			log.Fatalf("error while closing file: %v", err)
		}
	}()

	rows, err := readParfumeData.GetRows("Template Impor Produk")
	if err != nil {
		log.Fatalf("error while reading rows: %v", err)
	}

	for i, row := range rows {
		if i < 4 { // skip header
			continue
		}
		if len(row) < 14 {
			continue
		}

		name := row[0]
		categoryName := row[1]
		brandName := row[2]
		typeName := row[3]
		description := row[4]
		favorite := row[5]
		image := row[6]
		isNew := row[7]
		price := row[13]

		priceML, err := strconv.ParseFloat(price, 64)
		if err != nil {
			fmt.Printf("gagal parsing harga di baris %d (%s): %v\n", i+1, name, err)
			continue
		}

		// bikin / cari relasi kategori
		var category repository.Categories
		db.FirstOrCreate(&category, repository.Categories{Name: categoryName})

		// bikin / cari relasi brand
		var brand repository.Brand
		db.FirstOrCreate(&brand, repository.Brand{
			Name:        brandName,
			Description: brandName,
			Logo:        "default.png",
		})

		// bikin / cari relasi type
		var pType repository.Type
		db.FirstOrCreate(&pType, repository.Type{Name: typeName})

		// konversi favorite & isNew
		isFav := favorite == "1" || favorite == "true" || favorite == "yes"
		isNewBool := isNew == "1" || isNew == "true" || isNew == "yes"

		
		parfume := repository.Parfume{
			Name:        name,
			Description: description,
			PriceML:     priceML,
			Image:       image,
			TypeID:      pType.ID,
			CategoryID:  category.ID,
			BrandID:     brand.ID,
			Favorite:    isFav,
			IsNew:       isNewBool,
		}

		if err := db.FirstOrCreate(&parfume, repository.Parfume{Name: name}).Error; err != nil {
			fmt.Printf("gagal insert %s: %v\n", name, err)
		} else {
			fmt.Printf("sukses insert: %s (%.0f/ml)\n", name, priceML)
		}
	}
}
