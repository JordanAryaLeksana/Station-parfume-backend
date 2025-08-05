package services

import (
	"backend/src/config"
	"backend/src/modules/parfume/models"
	"backend/src/repository"
	"fmt"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func AddingBrandToParfume()