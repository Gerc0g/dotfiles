#!/bin/bash
set -e

if ! grep -q "dotfiles/shell/_loader.zsh" ~/.zshrc 2>/dev/null; then
  echo "" >> ~/.zshrc
  echo "# === Personal Platform ===" >> ~/.zshrc
  echo "[ -f ~/dotfiles/shell/_loader.zsh ] && source ~/dotfiles/shell/_loader.zsh" >> ~/.zshrc
  echo "✓ Added loader to ~/.zshrc"
fi

echo "✅ dotfiles installed"
echo "Reload: source ~/.zshrc"
echo "Try: ?"
