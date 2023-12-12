# tg-dice-bot

## 部署
### 基于 Docker 进行部署
```shell
docker run --name tg-dice-bot -d --restart always \
-e SQL_DSN="root:123456@tcp(localhost:3306)/dice_bot" \
-e REDIS_CONN_STRING="redis://default:<password>@<addr>:<port>" \
-e TELEGRAM_API_TOKEN="683091xxxxxxxxxxxxxxxxywDuU" \
deanxv/tg-dice-bot
```
其中，`SQL_DSN`,`REDIS_CONN_STRING`,`TELEGRAM_API_TOKEN`修改为自己的。

### 部署到第三方平台

<details>
<summary><strong>部署到 Zeabur</strong></summary>
<div>

> Zeabur 的服务器在国外，自动解决了网络的问题，同时免费的额度也足够个人使用

1. 首先 fork 一份代码。
2. 进入 [Zeabur](https://zeabur.com?referralCode=deanxv)，登录，进入控制台。
3. 新建一个 Project，在 Service -> Add Service 选择 Marketplace，选择 MySQL，并记下连接参数（用户名、密码、地址、端口）。
4. 使用mysql视图化工具连接mysql，运行 ```create database `dice_bot` ``` 创建数据库。
5. 在 Service -> Add Service，选择 Git（第一次使用需要先授权），选择你 fork 的仓库。
6. Deploy 会自动开始，先取消。
7. 添加环境变量
   
   `SQL_DSN`:`<username>:<password>@tcp(<addr>:<port>)/dice_bot`

   `REDIS_CONN_STRING`:`redis://default:<password>@<addr>:<port>`

   `TELEGRAM_API_TOKEN`:`你的TG机器人的TOKEN`
   
   保存。
9. 选择 Redeploy。

</div>
</details>

## 配置

### 环境变量

1. `SQL_DSN`：`SQL_DSN=root:123456@tcp(localhost:3306)/dice_bot`
2. `REDIS_CONN_STRING`：`REDIS_CONN_STRING:redis://default:<password>@<addr>:<port>`
3. `TELEGRAM_API_TOKEN`：`683091xxxxxxxxxxxxxxxxywDuU` 你的TG机器人的TOKEN
