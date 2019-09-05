package controllers

import (
	"github.com/astaxie/beego"
	"strconv"
	"github.com/astaxie/beego/orm"
	"BeegoFresh/models"
	"github.com/gomodule/redigo/redis"
	"time"
	"strings"
	"fmt"
	"github.com/smartwalle/alipay"
)

type OrderController struct {
	beego.Controller
}

func(this*OrderController)ShowOrder(){
	//获取数据
	//来自购物车页面：cart.html
	//skuid为选中的goodsSku的id
	skuids :=this.GetStrings("skuid")
	beego.Info(skuids)
	//校验数据
	if len(skuids) == 0{
		beego.Info("请求数据错误")
		this.Redirect("/user/cart",302)
		return
	}

	//处理数据
	o := orm.NewOrm()
	conn,_ := redis.Dial("tcp","192.168.150.20:6379")
	defer conn.Close()
	//获取用户数据
	var user models.User
	userName := this.GetSession("userName")
	user.Name = userName.(string)
	o.Read(&user,"Name")

	goodsBuffer := make([]map[string]interface{},len(skuids))

	totalPrice := 0
	totalCount := 0
	for index,skuid := range skuids{
		temp := make(map[string]interface{})

		id ,_ := strconv.Atoi(skuid)
		//查询商品数据
		var goodsSku models.GoodsSKU
		goodsSku.Id = id
		o.Read(&goodsSku)

		temp["goods"] = goodsSku
		//获取商品数量
		count,_ :=redis.Int(conn.Do("hget","cart_"+strconv.Itoa(user.Id),id))
		temp["count"] = count
		//计算小计
		amount := goodsSku.Price * count
		temp["amount"] = amount

		//计算总金额和总件数
		totalCount += count
		totalPrice += amount

		goodsBuffer[index] = temp
	}

	this.Data["goodsBuffer"] = goodsBuffer

	//获取地址数据
	var addrs []models.Address
	o.QueryTable("Address").RelatedSel("User").Filter("User__Id",user.Id).All(&addrs)
	this.Data["addrs"] = addrs

	//传递总金额和总件数
	this.Data["totalPrice"] = totalPrice
	this.Data["totalCount"] = totalCount
	transferPrice := 10
	this.Data["transferPrice"] = transferPrice
	this.Data["realyPrice"] = totalPrice + transferPrice

	//传递所有商品的id
	this.Data["skuids"] = skuids

	//返回视图
	this.TplName = "place_order.html"
}

//添加订单
func(this*OrderController)AddOrder(){
	//获取数据
	addrid,_ :=this.GetInt("addrid")
	payId,_ :=this.GetInt("payId")
	skuid := this.GetString("skuids")
	ids := skuid[1:len(skuid)-1]

	skuids := strings.Split(ids," ")


	beego.Error(skuids)
	//totalPrice,_ := this.GetInt("totalPrice")
	totalCount,_ := this.GetInt("totalCount")
	transferPrice,_ :=this.GetInt("transferPrice")
	realyPrice,_:=this.GetInt("realyPrice")


	resp := make(map[string]interface{})
	defer this.ServeJSON()
	//校验数据
	if len(skuids) == 0{
		resp["code"] = 1
		resp["errmsg"] = "数据库链接错误"
		this.Data["json"] = resp
		return
	}
	//处理数据
	//向订单表中插入数据
	o := orm.NewOrm()

	o.Begin()//标识事务的开始

	userName := this.GetSession("userName")
	var user models.User
	user.Name = userName.(string)
	o.Read(&user,"Name")

	var order models.OrderInfo
	order.OrderId = time.Now().Format("2006010215030405")+strconv.Itoa(user.Id)
	order.User = &user
	order.Orderstatus = 1
	order.PayMethod = payId
	order.TotalCount = totalCount
	order.TotalPrice = realyPrice
	order.TransitPrice = transferPrice

	//查询地址
	var addr models.Address
	addr.Id = addrid
	o.Read(&addr)

	order.Address = &addr

	//执行插入操作
	o.Insert(&order)


	//想订单商品表中插入数据
	conn,_ :=redis.Dial("tcp","192.168.150.20:6379")

	for _,skuid := range skuids{
		id,_ := strconv.Atoi(skuid)

		var goods models.GoodsSKU
		goods.Id = id
		i := 3

		for i> 0{
		o.Read(&goods)

		var orderGoods models.OrderGoods

		orderGoods.GoodsSKU = &goods
		orderGoods.OrderInfo = &order

		count ,_ :=redis.Int(conn.Do("hget","cart_"+strconv.Itoa(user.Id),id))

		if count > goods.Stock{
			resp["code"] = 2
			resp["errmsg"] = "商品库存不足"
			this.Data["json"] = resp
			o.Rollback()  //标识事务的回滚
			return
		}

		preCount := goods.Stock

		time.Sleep(time.Second * 5)
		beego.Info(preCount,user.Id)

		orderGoods.Count = count

		orderGoods.Price = count * goods.Price

		o.Insert(&orderGoods)

		goods.Stock -= count
		goods.Sales += count

		updateCount,_:=o.QueryTable("GoodsSKU").Filter("Id",goods.Id).Filter("Stock",preCount).Update(orm.Params{"Stock":goods.Stock,"Sales":goods.Sales})
		if updateCount == 0{
			if i >0 {
				i -= 1
				continue
			}
			resp["code"] = 3
			resp["errmsg"] = "商品库存改变,订单提交失败"
			this.Data["json"] = resp
			o.Rollback()  //标识事务的回滚
			return
		}else{
			conn.Do("hdel","cart_"+strconv.Itoa(user.Id),goods.Id)
			break
		}
		}

	}

	//返回数据
	o.Commit()  //提交事务
	resp["code"] = 5
	resp["errmsg"] = "ok"
	this.Data["json"] = resp

}

//处理支付
func(this*OrderController)HandlePay(){

	var aliPublicKey =  "MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAuiR8FN1JCfgPTtCNRnN+"+
						"s8o50N4FhrTddix+dTI3kHlOS2Vm9pdXZdI3yNeR5UQp/AXVPbNG0/GgeLsBCMcc"+
						"OMdsmWzs5/7BTH6KaM8AXT0G5S6WnpxrKnNcR6TcqFFKctXRvMVIuxaGDFZzwQ7B"+
						"Cv6Eb462agUoxGfsgRnZq4AWYNMT++UlqhOh5+faIdaFicI7+Q/Cz0bSm4gNG3QD"+
						"uROvX/xt98s84pINFomubXk1XChi5nBRpk/GcS132WF/IIeGg9zzlGS+UmeHha6N"+
						"WSaJqta8Pi+xySDsZuV4Bmlb9lO6hlRQjCA1XcFxih8WhtaVxio+BBmOuWkLSGMo"+
						"EQIDAQAB"
						// 可选，支付宝提供给我们用于签名验证的公钥，通过支付宝管理后台获取



	var privateKey = 	"MIIEogIBAAKCAQEAuiR8FN1JCfgPTtCNRnN+s8o50N4FhrTddix+dTI3kHlOS2Vm"+
						"9pdXZdI3yNeR5UQp/AXVPbNG0/GgeLsBCMccOMdsmWzs5/7BTH6KaM8AXT0G5S6W"+
						"npxrKnNcR6TcqFFKctXRvMVIuxaGDFZzwQ7BCv6Eb462agUoxGfsgRnZq4AWYNMT"+
						"++UlqhOh5+faIdaFicI7+Q/Cz0bSm4gNG3QDuROvX/xt98s84pINFomubXk1XChi"+
						"5nBRpk/GcS132WF/IIeGg9zzlGS+UmeHha6NWSaJqta8Pi+xySDsZuV4Bmlb9lO6"+
						"hlRQjCA1XcFxih8WhtaVxio+BBmOuWkLSGMoEQIDAQABAoIBAEDs9uakJJS4lEZO"+
						"UeiP4lK9p5rlxhGtRk2wyU8BfAYFebysmtRKB275ewGvxzCrrOU995n0zTCh5/IU"+
						"cBDqujpHvOZq6tskrbGLtaVHBn5/Cceoj1q1fl+pKzfGFj0TVZ9mWyi3u12eRpVJ"+
						"FkjxZ335NqJeqdui/ww6n3CMXrMFehaDVGRXkkm8349k32vcnz6lB9kXuk+Ob3Oo"+
						"tVuuEOoebWiglBWMtliojc6oAQIA+etO3GcDgTA5I32H1TkaOXeVj4A65LV1ti3O"+
						"QlVHMIRr/nvvEWGo5VYRhe807Qrts8GcKOWA73j2xYMFud9fjKHDsd+V+LVKdW8s"+
						"6XtWWL0CgYEA842FOTPhV7f5y+hRbjypqPW6ZVf6mMqtIE5oZ4oRJlWnF/rdof7o"+
						"X3fdyCzRZlkbevJHLJmA42qH2L30VWOeqGRpClCVBQZqWaGgxXfSdK8W9BuaumXm"+
						"ZWC/0kbAjTf0RwXHE+M9n0Nrdt631Rt+0rrTY04vUQ35BciP/5JJTsMCgYEAw6fY"+
						"0z98tshTlhfoO55w+SqiBZ6F2Eio+7AWk0aD+U4+LSvH9lZBWlDszAcTJL8QFfOn"+
						"RXFvJIvrAdy2Uh/CHZ2zuSjIdAs2ewEol3r6UzHx5X/aFC5Kz3W67g9nM6tcvQ6Q"+
						"XQScuTbOoY9is3TTWuLMiddAqLjx0L4FwaBmKJsCgYBMjDBRGEM9BK/YLL5bPWm9"+
						"lu3sqEg0+Y6MVthtonFdcRJBcTFzluCGEPB918hAuMTwUXGZTO27jGIB90HyDItz"+
						"NYvmGAmeOLP4U9pp9g0Ja3Z1Zq+s4hYVyuC/QEmImQuHvwMg9w0JH3GJPNreefPU"+
						"W6/QyGQKv6+C59SKaPntbQKBgGRQLUH53fZU9U4SCdZvYJrPeeyJnzQJ5OHOIXT3"+
						"BXkP3Z8JQGeTR8SHkzD0O6Nudk/a8ZsQEpzZQ+9bevrWH49RqLC5MTUV/qPIL0ij"+
						"G68F/3DcQTJxnZeKVAH0UcRTCqQ/0FJwp+3qJLz+p+s8bZS+jYHqo9Mdp5WPp6Hj"+
						"nB0bAoGANm8PF2FsAUo0/hXSnVk+FbN+VXzGPUU6TK8axPmXg8UL5m6on0DZD28O"+
						"znJda6qIezohxOWpLsjYd3oEY8kZxFJR1VN9QdK7tFDoJks+2qnoK9cQuWHqPrrU"+
						"M7sZvYmbuVIA/8whnJlqWMkKmSaYSI+ggd7wYoAMctsMfHDouGA="
						// 必须，上一步中使用 RSA签名验签工具 生成的私钥

	var appId = "2016101400682775"
	client,err := alipay.New(appId, aliPublicKey, privateKey, false)

	// 将 key 的验证调整到初始化阶段
	if err != nil {
		fmt.Println(err)
		return
	}

	//获取数据
	orderId := this.GetString("orderId")
	totalPrice := this.GetString("totalPrice")

	var p = alipay.TradePagePay{}
	p.NotifyURL = "http://xxx"
	p.ReturnURL = "http://192.168.150.20:8080/user/payok"
	p.Subject = "天天生鲜购物平台"
	p.OutTradeNo = orderId
	p.TotalAmount = totalPrice
	p.ProductCode = "FAST_INSTANT_TRADE_PAY"

	url, err := client.TradePagePay(p)
	if err != nil {
		fmt.Println(err)
	}

	var payURL = url.String()
	this.Redirect(payURL,302)
}

//支付成功
func(this*OrderController)PayOk(){
	//获取数据
	//out_trade_no=999998888777
	orderId := this.GetString("out_trade_no")


	//校验数据
	if orderId ==""{
		beego.Info("支付返回数据错误")
		this.Redirect("/user/userCenterOrder",302)
		return
	}

	//操作数据

	o := orm.NewOrm()
	count,_:=o.QueryTable("OrderInfo").Filter("OrderId",orderId).Update(orm.Params{"Orderstatus":2})
	if count == 0{
		beego.Info("更新数据失败")
		this.Redirect("/user/userCenterOrder",302)
		return
	}

	//返回视图
	this.Redirect("/user/userCenterOrder",302)
}
