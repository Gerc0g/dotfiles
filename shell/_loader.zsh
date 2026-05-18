for f in ~/dotfiles/shell/*.zsh; do
  [ "$(basename "$f")" = "_loader.zsh" ] && continue
  source "$f"
done
