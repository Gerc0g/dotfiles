# WikiPedik project-memory helpers. The deterministic half lives in the core
# (`hq wiki`); the curator LLM half stays in the wiki skills. These functions
# keep the historical command names.

wiki-sync()              { hq wiki sync "$@" }
wiki-status()            { hq wiki status "$@" }
wiki-synthesize()        { hq wiki synthesize "$@" }
wiki-commit()            { hq wiki commit "$@" }
wiki-autocommit()        { hq wiki autocommit "$@" }
wiki-hot-refresh()       { hq wiki hot-refresh "$@" }
wiki-rules-sync()        { hq wiki rules-sync "$@" }
wiki-bootstrap-product() { hq wiki bootstrap "$@" }

wiki-git() {
  git -C "${WIKIPEDIK_ROOT:-$HOME/Desktop/WikiPedik}" "$@"
}
