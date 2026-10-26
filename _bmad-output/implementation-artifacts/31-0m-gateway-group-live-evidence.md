# Gateway-group owner: sanitized acceptance evidence

## Scope and versions

Consumer-neutral isolated appliance only; core **26.7.7**, Terraform Core
**1.16.5**, cached dev-toolchain **1.12.24**. Individual prerequisite gateways
were synthetic, non-default and monitoring-disabled. No operator inventory,
credentials, raw configuration, production endpoint or adoption plan is included.
The owner reconfigures interface routes, not PF; actual multi-WAN traffic
failover/load balancing, older core releases and OpenTofu remain unverified.

## Executed candidates and retained failures

| Candidate | Executed outcome |
| --- | --- |
| `23f2fd34e61d1c853d6d00fb664701771a10404df7e60e163c5dff222fd616c1` | Initial Core run failed before group creation because freshly created gateway names were not yet eligible in the upstream cached selector. Cleanup and exact restoration passed; not counted as lifecycle success. |
| `0a11cd949258e551010b9f0653975289ada2c4f501588b257240eefc12fa835d` | Pre-review 13-step lifecycle passed in 91.70 seconds, with real deletion, zero test entries and exact configuration/routes restoration. Superseded by review repairs and expanded proof. |
| `8cbd5db329e7f1fcb6698049279f2b6fd3258906435c8c96cb91bc91c71cf5af` | Repaired 16-step lifecycle passed in 135.08 seconds; eleven independent saved-field observations passed, followed by real Terraform deletion and exact restoration. An outer operator wrapper subsequently failed assigning a shell-reserved variable; the actual runner success marker, Core pass, zero counts, restored baselines and unchanged hashes were independently verified. The wrapper error remains retained, not hidden or presented as a clean outer-process exit. |

Additional selector/dependency calibration failures and unsupported first-tier
clear negative controls remain separate from passing lifecycle evidence.
Final publication-tree verification is recorded below after its frozen run.
This evidence artifact is excluded from the executable/source digest to avoid
self-reference; runtime sources, executable helpers and publication claim inputs
are included. Raw evidence and independently reviewed private helper manifests
remain private and are not public CI artifacts.

## Proof exercised

- Real Core create/read, initially empty first tier with nonempty tier2, update
  while first tier remains empty, then populated first tier with stable UUID.
- UUID data-source lookup; actual Core import and exact no-change plans.
- Tier swaps, tiers3-5 population and subsequent clears, trigger, sticky/ordinary/
  default pool selections and description clearing. Independent saved-model
  observations compare every owned field, not only provider-derived attributes.
- Populated first-tier clear rejected before any write, with the same UUID and
  unchanged authoritative fields. This is an explicit upstream limitation, not
  positive first-tier-clear or universal tier-clear completeness proof.
- External drift detected and repaired. A disabled official-API dependency on the
  same tracked UUID caused actual Core deletion rejection; unlinking preserved
  identity and allowed later real deletion. The fixture does not activate PF.
- Real Terraform destroy followed by independent searches for test-owned groups,
  gateways and references: zero before baseline rollback and again afterward.
  Full saved configuration and IPv4/IPv6 routes restored exactly; source, guard,
  oracle and independently bound host helper sources remained unchanged.
- API credentials flow through an acceptance-only fixed-upstream proxy that pins
  every actual TLS connection before forwarding HTTP bytes. Local substitution
  regression verifies a changed certificate receives no credentials. Expected
  helper hashes are checked before either helper executes; changed/missing hashes
  fail closed. These are acceptance controls, not new provider settings.

## Offline and review gates

Parent container verification passed 30 named gateway-group root tests with race
detection, five actual local Terraform Core failure/private-replay/server-restart
cases, the full fresh Go suite, two helper security regression methods, two
identical generation passes and all six default quality targets. No failing check
was suppressed. The local fault API is synthetic and is not appliance failure
injection evidence. Every computed lookup field and unsupported-create zero-write
behavior have separate protocol assertions.

Three independent non-implementing reviewers covered four initial review layers;
one also performed the alignment layer, including an exact-contract supplement.
All observations were individually triaged before repairs. Two reviewers then
rechecked the repaired recovery and verification/security paths and reported no
remaining reproducible defects in that bounded follow-up. The initial upstream
first-tier-clear completeness gap remains explicitly open.

## Final publication tree

Publication digest **`a7bcb657cd57f7bb454f93d508a49c84f905b87d6c143d00bd97ad340b01983d`** passed the complete
16-step lifecycle in **136.26s**, with **11/11 independent saved-field
observations**, actual Terraform deletion, zero test-owned groups/gateways/
references before and after rollback, exact full configuration and IPv4/IPv6
route restoration, unchanged source/helper hashes, and a clean runner/outer exit
**0**. The earlier wrapper-only error is retained separately above.

The final publication tree passed **6/6 quality targets**, deterministic generation,
whitespace checks, publication-file privacy inspection and a **37-file directory
secret scan with zero findings**. Parent's full race/Go/Core and helper-security
results are recorded above; hosted PR CI is verified separately, not inferred
from local checks. No release or production adoption is implied.
