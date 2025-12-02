# Deploy Go WebSocket Server to Render

## Step 1: Push to GitHub

1. Initialize git (if not already):
```bash
cd location_tracker
git init
git add .
git commit -m "Initial commit: Go WebSocket server"
```

2. Create a new repository on GitHub and push:
```bash
git remote add origin https://github.com/HaShiRaMaReiZo/location-tracker.git
git branch -M main
git push -u origin main
```

## Step 2: Deploy on Render

1. **Go to Render Dashboard**: https://dashboard.render.com

2. **Create New Web Service**:
   - Click "New +" → "Web Service"
   - Connect your GitHub repository
   - Select the `location_tracker` repository

3. **Configure Service**:
   - **Name**: `location-tracker` (or any name)
   - **Environment**: `Go`
   - **Build Command**: `go build -o location_tracker`
   - **Start Command**: `./location_tracker`
   - **Port**: `8080` (Render will set PORT automatically)

4. **Environment Variables**:
   - `PORT`: Leave empty (Render sets this automatically)
   - `LARAVEL_API_URL`: `https://ok-delivery.onrender.com`

5. **Click "Create Web Service"**

## Step 3: Get Your WebSocket URL

After deployment, Render will give you a URL like:
```
https://location-tracker-xxxx.onrender.com
```

**Important**: Render free tier services spin down after 15 minutes of inactivity. For production, consider:
- Using a paid plan (always-on)
- Or use a service like Railway, Fly.io, or DigitalOcean

## Step 4: Update Laravel Configuration

Add to your Laravel `.env` on Render:
```env
GO_WEBSOCKET_URL=https://location-tracker-xxxx.onrender.com
```

Or if using Render's internal networking:
```env
GO_WEBSOCKET_URL=http://location-tracker:8080
```

## Step 5: Update Flutter Client

In your Flutter app, update the WebSocket URL:
```dart
LocationWebSocketService(
  packageId: package.id,
  userId: userId,
  userRole: 'merchant',
  merchantId: merchantId,
  baseUrl: 'wss://location-tracker-xxxx.onrender.com', // Use wss:// for HTTPS
)
```

## Alternative: Deploy as Separate Service on Same Render Account

If both Laravel and Go are on Render:

1. Deploy Go server as shown above
2. Use Render's internal service URL: `http://location-tracker:8080`
3. This avoids external HTTPS and is faster

## Testing

1. **Check health endpoint**:
```bash
curl https://location-tracker-xxxx.onrender.com/health
```

2. **Test WebSocket connection**:
```bash
# Using wscat
wscat -c "wss://location-tracker-xxxx.onrender.com/ws?user_id=1&role=merchant&merchant_id=1&package_id=123"
```

3. **Test location update**:
```bash
curl -X POST https://location-tracker-xxxx.onrender.com/api/location/update?package_status=on_the_way \
  -H "Content-Type: application/json" \
  -d '{
    "rider_id": 1,
    "latitude": 16.8661,
    "longitude": 96.1951,
    "package_id": 123,
    "last_update": "2024-01-01T12:00:00Z"
  }'
```

## Troubleshooting

### Service Spins Down (Free Tier)
- Render free tier services sleep after 15 min inactivity
- First request after sleep takes ~30 seconds (cold start)
- Solution: Upgrade to paid plan or use keep-alive ping

### WebSocket Connection Fails
- Make sure you're using `wss://` (not `ws://`) for HTTPS
- Check CORS settings if needed
- Verify the service is running in Render dashboard

### Location Not Broadcasting
- Check Laravel logs for errors sending to Go server
- Verify `GO_WEBSOCKET_URL` is set correctly
- Check package status is `on_the_way` (not `ready_for_delivery`)

