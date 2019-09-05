package main

import (
	_ "BeegoFresh/routers"
	"github.com/astaxie/beego"
	_ "BeegoFresh/models"
)

func main() {
	beego.Run()
}

