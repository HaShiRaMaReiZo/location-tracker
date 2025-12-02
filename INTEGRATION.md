# Integration Guide: Go WebSocket Server + Laravel + Flutter

## Architecture Overview

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐      ┌──────────────┐
│   Rider     │─────▶│   Laravel    │─────▶│  Go Server  │─────▶│   Flutter    │
│    App      │ POST │     API      │ HTTP │  WebSocket  │  WS  │   Clients    │
└─────────────┘      └──────────────┘      └─────────────┘      └──────────────┘
```

## Step 1: Start Go WebSocket Server

```bash
cd location_tracker
go mod download
go run main.go
```

Server will start on `http://localhost:8080`

## Step 2: Configure Laravel

Add to `.env`:
```env
GO_WEBSOCKET_URL=http://localhost:8080
```

The Laravel `LocationController` will automatically send location updates to the Go server.

## Step 3: Update Flutter Client

### Add WebSocket dependency

In `ok_delivery/pubspec.yaml`:
```yaml
dependencies:
  web_socket_channel: ^2.4.0
```

### Create WebSocket Service

Create `ok_delivery/lib/services/location_websocket_service.dart`:

```dart
import 'dart:async';
import 'dart:convert';
import 'package:web_socket_channel/web_socket_channel.dart';
import '../models/package_model.dart';

class LocationWebSocketService {
  WebSocketChannel? _channel;
  StreamController<Map<String, dynamic>>? _locationController;
  Timer? _reconnectTimer;
  bool _isConnecting = false;

  final int packageId;
  final int userId;
  final String userRole;
  final int? merchantId;
  final String baseUrl;

  LocationWebSocketService({
    required this.packageId,
    required this.userId,
    required this.userRole,
    this.merchantId,
    this.baseUrl = 'ws://localhost:8080',
  }) {
    _locationController = StreamController<Map<String, dynamic>>.broadcast();
  }

  Stream<Map<String, dynamic>> get locationStream => 
      _locationController!.stream;

  Future<void> connect() async {
    if (_isConnecting || _channel != null) return;
    
    _isConnecting = true;
    try {
      final uri = Uri.parse(
        '$baseUrl/ws?user_id=$userId&role=$userRole'
        '${merchantId != null ? '&merchant_id=$merchantId&package_id=$packageId' : ''}'
      );

      _channel = WebSocketChannel.connect(uri);
      
      _channel!.stream.listen(
        (message) {
          try {
            final data = jsonDecode(message);
            if (data['event'] == 'rider.location.updated') {
              _locationController?.add(data['data'] as Map<String, dynamic>);
            }
          } catch (e) {
            print('Error parsing WebSocket message: $e');
          }
        },
        onError: (error) {
          print('WebSocket error: $error');
          _reconnect();
        },
        onDone: () {
          print('WebSocket closed');
          _reconnect();
        },
      );
      
      _isConnecting = false;
    } catch (e) {
      _isConnecting = false;
      print('Failed to connect: $e');
      _reconnect();
    }
  }

  void _reconnect() {
    _reconnectTimer?.cancel();
    _reconnectTimer = Timer(const Duration(seconds: 5), () {
      connect();
    });
  }

  void disconnect() {
    _reconnectTimer?.cancel();
    _channel?.sink.close();
    _channel = null;
    _locationController?.close();
  }
}
```

### Update LiveTrackingMapScreen

Replace polling with WebSocket:

```dart
LocationWebSocketService? _wsService;

@override
void initState() {
  super.initState();
  _loadLocation();
  
  // Connect WebSocket for real-time updates
  if (_isInTransit(widget.package.status)) {
    _wsService = LocationWebSocketService(
      packageId: widget.package.id,
      userId: _getUserId(), // Get from auth
      userRole: 'merchant',
      merchantId: _getMerchantId(), // Get from auth
    );
    
    _wsService!.locationStream.listen((data) {
      setState(() {
        _riderLatitude = data['latitude']?.toDouble();
        _riderLongitude = data['longitude']?.toDouble();
        _riderName = data['rider_name']?.toString();
        _lastUpdate = DateTime.parse(data['last_update']);
      });
    });
    
    _wsService!.connect();
  }
}

@override
void dispose() {
  _wsService?.disconnect();
  super.dispose();
}
```

## Testing

1. **Start Go server:**
```bash
cd location_tracker && go run main.go
```

2. **Test WebSocket connection:**
```bash
# Using wscat (install: npm install -g wscat)
wscat -c "ws://localhost:8080/ws?user_id=1&role=merchant&merchant_id=1&package_id=123"
```

3. **Send test location update:**
```bash
curl -X POST http://localhost:8080/api/location/update \
  -H "Content-Type: application/json" \
  -d '{
    "rider_id": 1,
    "latitude": 16.8661,
    "longitude": 96.1951,
    "package_id": 123,
    "last_update": "2024-01-01T12:00:00Z"
  }'
```

## Production Deployment

1. **Build Go server:**
```bash
GOOS=linux GOARCH=amd64 go build -o location_tracker
```

2. **Run as service (systemd):**
```ini
[Unit]
Description=Location Tracker WebSocket Server
After=network.target

[Service]
Type=simple
User=www-data
WorkingDirectory=/var/www/location_tracker
ExecStart=/var/www/location_tracker/location_tracker
Restart=always

[Install]
WantedBy=multi-user.target
```

3. **Nginx reverse proxy:**
```nginx
location /ws {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

## Performance Monitoring

Monitor Go server:
- CPU usage: `top -p $(pgrep location_tracker)`
- Connections: Check logs for connection count
- Memory: `ps aux | grep location_tracker`

