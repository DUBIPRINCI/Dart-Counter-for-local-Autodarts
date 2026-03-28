#!/bin/bash
set -e

INSTALL_DIR="/opt/dartcounter"
BINARY="dartcounter"
SERVICE_NAME="dartcounter"
GO_VERSION="1.26.1"
GO_BIN="/usr/local/go/bin/go"

echo "=== DartCounter Installation ==="
echo "Répertoire courant : $(pwd)"

# Vérifier qu'on est bien dans le dossier du projet
if [ ! -f "main.go" ]; then
    echo "ERREUR : Lancez le script depuis la racine du projet (là où se trouve main.go)"
    echo "Exemple : cd ~/DartCounter && bash deploy/install.sh"
    exit 1
fi

# ---- 1. Installer Go si absent ----
if [ ! -f "$GO_BIN" ]; then
    echo "Go non trouvé, installation de Go $GO_VERSION..."
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    echo "Go installé"
else
    echo "Go trouvé"
fi

# Forcer Go dans le PATH pour cette session
export PATH=$PATH:/usr/local/go/bin
echo "Version Go : $($GO_BIN version)"

# ---- 2. Compiler depuis les sources ----
echo "Compilation en cours (peut prendre 1-2 minutes)..."
$GO_BIN build -ldflags "-s -w" -o "$BINARY" .

if [ ! -f "$BINARY" ]; then
    echo "ERREUR : La compilation a échoué, le binaire '$BINARY' n'existe pas"
    exit 1
fi
echo "Compilation OK -> $(ls -lh $BINARY)"

# ---- 3. Créer l'utilisateur système ----
if ! id "$SERVICE_NAME" &>/dev/null; then
    sudo useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_NAME"
    echo "Utilisateur système créé : $SERVICE_NAME"
fi

# ---- 4. Créer les répertoires ----
sudo mkdir -p "$INSTALL_DIR"/{sounds/default,data}

# ---- 5. Copier le binaire ----
sudo cp "$BINARY" "$INSTALL_DIR/"
sudo chmod +x "$INSTALL_DIR/$BINARY"
echo "Binaire installé dans $INSTALL_DIR"

# ---- 6. Copier les sons ----
if [ -d "sounds" ]; then
    sudo cp -r sounds/* "$INSTALL_DIR/sounds/"
    echo "Sons copiés"
fi

# ---- 7. Permissions ----
sudo chown -R "$SERVICE_NAME:$SERVICE_NAME" "$INSTALL_DIR"

# ---- 8. Installer le service systemd ----
sudo cp deploy/dartcounter.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"
sudo systemctl restart "$SERVICE_NAME"

echo ""
echo "=== Installation terminée ==="
echo "DartCounter tourne sur http://localhost:8080"
echo ""
echo "Commandes utiles :"
echo "  sudo systemctl status dartcounter"
echo "  sudo systemctl restart dartcounter"
echo "  sudo journalctl -u dartcounter -f"
