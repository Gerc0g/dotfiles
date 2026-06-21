# ML And Quant Workflow

Reference for ML, NLP, data science, quant, backtesting, metrics, leakage,
reproducibility, notebooks, MLflow, W&B, model evaluation, and financial
research tasks.

## Defaults

- Notebook is exploration only. Anything committed should be scriptable.
- Ask the user which metric matters; do not invent the business/evaluation
  metric.
- Pin versions in project files (`pyproject.toml`, lock files).
- Seed randomness and hash or record data inputs for runs worth keeping.
- Log meaningful runs to the repo's experiment tracker (MLflow/W&B) when it has
  that convention. Note: dev-stack does NOT run a managed MLflow — it provides
  Langfuse (`$DEV_STACK_HOST:3001`) for LLM tracing and a MinIO `mlflow` bucket
  for artifacts; run MLflow yourself if a project needs the tracking server.

## Leakage Checks

Flag immediately:

- train/test contamination;
- target leakage;
- look-ahead bias in time series or trading;
- scaling/feature engineering fit on full dataset;
- suspicious metrics such as Sharpe > 4 on a toy strategy or 0% drawdown.

## Quant Specifics

- Respect chronological splits.
- Treat fees, slippage, liquidity, and survivorship bias as first-class risks.
- Do not inspect or optimize against the golden test set.
- Explain assumptions and residual risk in final output.

Deep reference: `~/dotfiles/agent-profiles/PLATFORM.md` section "Quant / ML
specifics".
