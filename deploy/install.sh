#!/bin/bash
set -e

INSTALL_DIR="/opt/dartcounter"
BINARY="dartcounter"
SERVICE_NAME="dartcounter"

echo "=== DartCounter Installation ==="

# Create system user
if ! id "$SERVICE_NAME" &>/dev/null; then
    sudo useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_NAME"
    echo "Created system user: $SERVICE_NAME"
fi

# Create directories
sudo mkdir -p "$INSTALL_DIR"/{sounds/default,data}

# Copy binary
sudo cp "$BINARY" "$INSTALL_DIR/"
sudo chmod +x "$INSTALL_DIR/$BINARY"

# Copy sounds
if [ -d "sounds" ]; then
    sudo cp -r sounds/* "$INSTALL_DIR/sounds/"
fi

# Set ownership
sudo chown -R "$SERVICE_NAME:$SERVICE_NAME" "$INSTALL_DIR"

# Install systemd service
sudo cp deploy/dartcounter.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl start "$SERVICE_NAME"

echo "=== Installation complete ==="
echo "DartCounter is running at http://localhost:8080"
echo "Manage with: sudo systemctl {start|stop|restart|status} $SERVICE_NAME"
