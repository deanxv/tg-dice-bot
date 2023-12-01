# tg-dice-bot

## 部署

### 部署到第三方平台

<details>
<summary><strong>部署到 Zeabur</strong></summary>
<div>

> Zeabur 的服务器在国外，自动解决了网络的问题，同时免费的额度也足够个人使用

1. 首先 fork 一份代码。
2. 进入 [Zeabur](https://zeabur.com?referralCode=deanxv)，登录，进入控制台。
3. 新建一个 Project，在 Service -> Add Service 选择 Marketplace，选择 MySQL，并记下连接参数（用户名、密码、地址、端口）。
4. 复制链接参数，运行 ```create database `dice_bot` ``` 创建数据库。
5. 然后在 Service -> Add Service，选择 Git（第一次使用需要先授权），选择你 fork 的仓库。
6. Deploy 会自动开始，先取消。添加一个 `SQL_DSN`，值为 `<username>:<password>@tcp(<addr>:<port>)/dice_bot`
   ，再添加一个 `TELEGRAM_API_TOKEN`，值为 `你的TG机器人的TOKEN`，然后保存。
7. 选择 Redeploy。

</div>
</details>

## 配置

### 环境变量

1. `SQL_DSN`：`SQL_DSN=root:123456@tcp(localhost:3306)/dice_bot`
2. `TELEGRAM_API_TOKEN`：`683091xxxxxxxxxxxxxxxxywDuU` 你的TG机器人的TOKEN
