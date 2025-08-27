package seeders

import (
	"backend/src/repository"
	"fmt"
	"strconv"
	"strings"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func SeedBottles(db *gorm.DB) {
	readBottlesData, err := excelize.OpenFile("C:\\Webdev\\station-parfume\\Produk_Station_Parfume.xlsx")
	if err != nil {
		fmt.Printf("error while parsing data: %v\n", err)
		return
	}
	defer func() {
		if err := readBottlesData.Close(); err != nil {
			fmt.Printf("error while closing file: %v\n", err)
		}
	}()

	rows, err := readBottlesData.GetRows("BOTOL")
	if err != nil {
		fmt.Printf("error while reading rows: %v\n", err)
		return
	}

	for i, row := range rows {
		if i < 1 { 
			continue
		}
		if len(row) < 14 {
			continue
		}

		nameBottle := row[0]
		typeBottle := row[1]
		descriptionBottle := row[2]
		imageBottle := row[3]
		sizeBottle:= row[10]
		isNewBottle := strings.ToLower(strings.TrimSpace(row[4]))
		favoriteBottle := strings.ToLower(strings.TrimSpace(row[6]))
		price := row[13]

	
		priceFloat, err := strconv.ParseFloat(price, 64)
		if err != nil {
			fmt.Printf(" Gagal parsing harga di baris %d (%s): %v\n", i+1, nameBottle, err)
			continue
		}

		sizeFloat, err := strconv.ParseFloat(sizeBottle, 64)
		if err != nil {
			sizeFloat = 0
		}

		var typeBottleModel repository.TypeBottle
		if err := db.Where("name = ?", typeBottle).First(&typeBottleModel).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				typeBottleModel = repository.TypeBottle{Name: typeBottle}
				db.Create(&typeBottleModel)
			} else {
				fmt.Printf("❌ Error mencari type_bottle %s: %v\n", typeBottle, err)
				continue
			}
		}

		// convert string "yes"/"1"/"true" jadi bool
		isNew := isNewBottle == "yes" || isNewBottle == "1" || isNewBottle == "true"
		isFav := favoriteBottle == "yes" || favoriteBottle == "1" || favoriteBottle == "true"

		bottle := repository.Bottle{
			Name:        nameBottle,
			Description: descriptionBottle,
			Price:       priceFloat,
			Image:       imageBottle,
			Size:        sizeFloat,
			IsNew:       isNew,
			IsFavorite:  isFav,
			TypeBottleID: typeBottleModel.ID,
		}

		if err := db.Create(&bottle).Error; err != nil {
			fmt.Printf("❌ Error insert bottle %s: %v\n", nameBottle, err)
		} else {
			fmt.Printf("✅ Berhasil insert bottle: %s\n", nameBottle)
		}
	}
}
