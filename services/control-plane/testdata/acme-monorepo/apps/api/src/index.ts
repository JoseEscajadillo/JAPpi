import express from "express"
import cors from "cors"
import Redis from "ioredis"

const app = express()
app.use(cors({ origin: process.env.CORS_ORIGIN }))
const redis = new Redis(process.env.REDIS_URL)
app.listen(process.env.PORT)
