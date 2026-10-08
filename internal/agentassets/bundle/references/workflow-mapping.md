# Workflow mapping cases

Workflow files are evidence, not an importer input. Enumerate workflows by structure, inspect commands and local actions, choose a bounded check scope, and record omissions. Each mapping below is a complete v2 config accepted by the same parser; source and omission notes are kept here so mappings cannot add non-schema fields.

| Case | Fixture | Mapping | Selected scope and omissions |
| --- | --- | --- | --- |
| Nonstandard names | `../assets/workflows/nonstandard-names.yml` | `../assets/mappings/nonstandard-names.json` | Select the explicit verify job and preserve its shell commands; names are not inferred from `build` or `test`. |
| Shared script | `../assets/workflows/shared-script.yml` | `../assets/mappings/shared-script.json` | Select `sh scripts/check.sh`; inspect the script as data and do not execute it while configuring. |
| Local composite | `../assets/workflows/local-composite.yml` | `../assets/mappings/local-composite.json` | Inline only the reviewed composite steps; omit unreviewed inputs and action side effects. |
| Multiple workflows | `../assets/workflows/multi-workflow.yml` | `../assets/mappings/multi-workflow.json` | Select the requested test workflow; omit release and deployment workflows. |
| Monorepo | `../assets/workflows/monorepo.yml` | `../assets/mappings/monorepo.json` | Select the stated working-directory package; report sibling packages omitted. |
| Partial features | `../assets/workflows/partial-features.yml` | `../assets/mappings/partial-features.json` | Map needs and ordinary commands; record matrix, service, secret, and expression values as assumptions. |
| No workflow / manifest only | — | `../assets/mappings/no-workflow.json` | Use manifest evidence only; do not invent CI parity or a check script. |

Matrix expansion, services, secrets, dynamic expressions, conditional needs, caches, permissions, artifacts, and deployment behavior require explicit assumptions or omission notes. A mixed-language repository uses one image containing every selected tool; v2 has one runner image.
