# Quick Setup Guide

## ✅ What's Done

1. ✅ Go WebSocket server created
2. ✅ Laravel integration updated
3. ✅ Status check: Only `on_the_way` shows location to merchants
4. ✅ Render deployment files created

## 🚀 Deployment Steps

### 1. Push to GitHub

```bash
cd location_tracker
git init
git add .
git commit -m "Go WebSocket server for location tracking"
git remote add origin https://github.com/yourusername/location-tracker.git
git push -u origin main
```

### 2. Deploy on Render

1. Go to https://dashboard.render.com
2. Click "New +" → "Web Service"
3. Connect your GitHub repo
4. Configure:
   - **Name**: `location-tracker`
   - **Environment**: `Go`
   - **Build Command**: `go build -o location_tracker`
   - **Start Command**: `./location_tracker`
5. Add environment variable:
   - `LARAVEL_API_URL`: `https://ok-delivery.onrender.com`
6. Click "Create Web Service"

### 3. Get Your WebSocket URL

After deployment, you'll get a URL like:
```
https://location-tracker-xxxx.onrender.com
```

### 4. Update Laravel .env on Render

In your Laravel service on Render, add:
```env
GO_WEBSOCKET_URL=https://location-tracker-xxxx.onrender.com
```

### 5. No Commands Needed!

The Go server will:
- ✅ Start automatically when deployed
- ✅ Listen on the PORT Render provides
- ✅ Accept WebSocket connections at `/ws`
- ✅ Accept location updates from Laravel at `/api/location/update`

## 📋 Status Rules

- **Merchants**: Can see rider location ONLY when package status = `on_the_way`
- **Office**: Can always see all rider locations

## 🧪 Test It

1. **Health check**:
```bash
curl https://location-tracker-xxxx.onrender.com/health
```

2. **Test WebSocket** (use wscat):
```bash
wscat -c "wss://location-tracker-xxxx.onrender.com/ws?user_id=1&role=merchant&merchant_id=1&package_id=123"
```

## ⚠️ Important Notes

1. **Render Free Tier**: Services sleep after 15 min inactivity. First request after sleep takes ~30 seconds.

2. **WebSocket URL**: Use `wss://` (not `ws://`) for HTTPS connections.

3. **Package Status**: Location only broadcasts to merchants when status is `on_the_way`.

## 🔧 Next Steps

After deployment, update your Flutter app to connect to the WebSocket server. See `INTEGRATION.md` for Flutter client implementation.

