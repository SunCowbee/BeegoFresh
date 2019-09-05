BeegoFresh

#ubuntu

#go

#beego

$ go get -u -v github.com/astaxie/beego

$ go get -u -v github.com/beego/bee

##配置beego执行文件环境变量：$GOPATH/bin

	$ vim .bashrc
  
	//在最后一行插入
  
	export PATH="$GOPATH/bin:$PATH"
  
	//然后保存退出
  
	$ source .bashrc
  
#mysql

#mysql-driver

go get -u -v github.com/go-sql-driver/mysql

#database

/etc/mysql/mysql.conf.d

sudo vim mysqld.cnf

transaction-isolation=READ-COMMITTED

mysql -uroot -proot

drop database dailyfresh;

create database dailyfresh charset=utf8;

use dailyfresh;

source dailyfresh.sql;

#SMTP

qq:hdulrbptfhnuidfc

#redis

192.168.150.20:6379

cd /etc/redis/

sudo redis-server /etc/redis/redis.conf 

#FastDFS

sudo  fdfs_trackerd  /etc/fdfs/tracker.conf

sudo fdfs_storaged  /etc/fdfs/storage.conf

#nginx

sudo  /usr/local/nginx/sbin/nginx

#alipay

https://github.com/smartwalle/alipay

go get -u -v github.com/smartwalle/alipay
