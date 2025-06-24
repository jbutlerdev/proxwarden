#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/proxwarden"
LOG_DIR="/var/log/proxwarden"
DATA_DIR="/var/lib/proxwarden"
SERVICE_FILE="/etc/systemd/system/proxwarden.service"
USER="proxwarden"
GROUP="proxwarden"

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

create_user() {
    if ! getent group "$GROUP" >/dev/null 2>&1; then
        print_info "Creating group: $GROUP"
        groupadd --system "$GROUP"
    fi

    if ! getent passwd "$USER" >/dev/null 2>&1; then
        print_info "Creating user: $USER"
        useradd --system --gid "$GROUP" --home-dir /nonexistent \
                --no-create-home --shell /bin/false "$USER"
    fi
}

create_directories() {
    print_info "Creating directories"
    
    mkdir -p "$CONFIG_DIR"
    mkdir -p "$LOG_DIR"
    mkdir -p "$DATA_DIR"
    
    chown "$USER:$GROUP" "$LOG_DIR"
    chown "$USER:$GROUP" "$DATA_DIR"
    chmod 755 "$CONFIG_DIR"
    chmod 750 "$LOG_DIR"
    chmod 750 "$DATA_DIR"
}

install_binary() {
    print_info "Installing ProxWarden binary"
    
    if [[ ! -f "./proxwarden" ]]; then
        print_error "ProxWarden binary not found. Please build it first with 'go build'"
        exit 1
    fi
    
    cp "./proxwarden" "$INSTALL_DIR/proxwarden"
    chmod 755 "$INSTALL_DIR/proxwarden"
    chown root:root "$INSTALL_DIR/proxwarden"
}

install_systemd_service() {
    print_info "Installing systemd service"
    
    if [[ ! -f "./systemd/proxwarden.service" ]]; then
        print_error "Systemd service file not found"
        exit 1
    fi
    
    cp "./systemd/proxwarden.service" "$SERVICE_FILE"
    chmod 644 "$SERVICE_FILE"
    
    systemctl daemon-reload
}

install_config() {
    if [[ -f "$CONFIG_DIR/proxwarden.yaml" ]]; then
        print_warn "Configuration file already exists, creating backup"
        cp "$CONFIG_DIR/proxwarden.yaml" "$CONFIG_DIR/proxwarden.yaml.backup.$(date +%Y%m%d-%H%M%S)"
    fi
    
    if [[ -f "./configs/proxwarden.example.yaml" ]]; then
        print_info "Installing example configuration"
        cp "./configs/proxwarden.example.yaml" "$CONFIG_DIR/proxwarden.yaml.example"
        
        if [[ ! -f "$CONFIG_DIR/proxwarden.yaml" ]]; then
            cp "./configs/proxwarden.example.yaml" "$CONFIG_DIR/proxwarden.yaml"
            print_warn "Please edit $CONFIG_DIR/proxwarden.yaml before starting the service"
        fi
    fi
    
    chown root:root "$CONFIG_DIR"/*.yaml*
    chmod 600 "$CONFIG_DIR"/proxwarden.yaml*
}

main() {
    print_info "Installing ProxWarden"
    
    check_root
    create_user
    create_directories
    install_binary
    install_systemd_service
    install_config
    
    print_info "Installation completed successfully!"
    print_info ""
    print_info "Next steps:"
    print_info "1. Edit the configuration: $CONFIG_DIR/proxwarden.yaml"
    print_info "2. Enable the service: systemctl enable proxwarden"
    print_info "3. Start the service: systemctl start proxwarden"
    print_info "4. Check status: systemctl status proxwarden"
    print_info "5. View logs: journalctl -u proxwarden -f"
}

main "$@"