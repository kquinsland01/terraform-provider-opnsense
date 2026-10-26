# Gateway-group stable API validation — 2026-10-25

## Outcome and boundary

Core 26.7.7 source review and non-skipped direct-API probes on a fresh disposable
NAT-only appliance establish a buildable API contract. No production endpoint,
real operator inventory or provider Terraform lifecycle is part of this evidence.
The missing provider owner is **not implemented** by this report. Story 31.0m owns
implementation, real Terraform import/no-change/drift/failure/cleanup proof and
review. Actual multi-WAN traffic failover/load balancing remains untested.

## Stable contract

Sources: tagged core [controller](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/controllers/OPNsense/Routing/Api/GroupSettingsController.php),
[model](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/models/OPNsense/Routing/GatewayGroups.xml)
and [validation/runtime adapter](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/models/OPNsense/Routing/GatewayGroups.php).

- `/api/routing/group_settings/{get,add,set,del,search,reconfigure}`, wrapper
  `gateway_group`, UUID item identity. Reconfigure is POST and returns `status=ok`;
  the controller invokes `interface routes configure`, not PF activation.
- Required unique `name`; tagged mask permits 1–32 ASCII letters/digits/underscore/
  hyphen despite its narrower validation-message wording. Existing names cannot
  be changed. Names may not collide with individual gateways.
- Five multi-select gateway-name tiers: `item`, `item2`, `item3`, `item4`, `item5`.
  GET returns selected maps; write selected keys, never labels or the GET map.
- Trigger keys: `down`, `downloss`, `downlatency`, `downlosslatency`; default `down`.
  Pool keys: empty/default, `round-robin`, `round-robin sticky-address`.
  `descr` is optional and clearable.
- Delete checks persisted gateway references. The tested disabled filter-rule
  reference caused HTTP **500** with a descriptive in-use error; the group and
  identity remained intact. A provider must not treat this as not-found/success.

## Executed observations

| Probe | Observed result |
|---|---|
| Two synthetic, non-default, monitoring-disabled individual gateways; group create/get with two tiers | Saved; exact names selected in the expected tier maps. Individual gateway reconfigure was required before the group selector admitted newly created members. |
| Group reconfigure | `status=ok`; management stayed reachable through isolated LAN forwarding. |
| Rename existing group | Rejected; original identity/name retained. |
| Swap tiers, trigger and pool update | Readback matched tier swap, `downlosslatency`, round-robin/sticky/default pool keys. |
| Clear second tier, pool and description | Readback matched the intended clears and retained the first tier. |
| Duplicate group name | Rejected. |
| Clear every tier | Returned `saved` but retained the previous first-tier selection on readback. Do **not** model this as a successful empty-group clear; reject all-empty planned tiers in the provider and test this normalization boundary. |
| Disabled filter rule referencing a separate group using an eligible existing lab gateway; delete group | In-use error; same UUID retained. Rule removal then allowed group deletion. |
| Final group/member deletion/reconfigure | Global searches had zero probe groups/gateways; exact IPv4/IPv6 routing snapshots matched the post-bootstrap baseline. |

## Failed controls retained, not counted as passes

Initial group attempts preceded individual-gateway runtime reconfigure and failed
member selection. Their temporary gateways were removed. A WAN-forwarded lab
management path was disrupted during routing reconfigure; it was replaced with
isolated LAN forwarding and the new baseline captured before the passing run.
One disabled-rule reference to a newly constructed group was rejected by the
rule's gateway selector; the successful dependency fixture above used a separately
eligible group and is not proof for every gateway status/cache combination.
No direct XML edits manufactured the dependency case. No raw configuration,
credentials, machine inventory or operational addressing is published here.

Controller SHA256: `315fbdaeeb46db1fc18f84ea2784fc7dde854b795ac62c7ac4ec609dc83f74d7`.
Model SHA256: `0bc2a1ceab2fd1bc5edb68e31da079eb2423f66e51c78245dca950b8779f78f4`.
Runtime adapter SHA256: `3fccf3f92c030910c99fd10235f91295c97625b059791a000e9a2df496046054`.

## Follow-up boundary: populated first-tier clear

A later isolated direct-API negative probe kept another tier populated while
attempting to clear an already-populated first tier. `item:""` returned `saved`
but retained the old first tier; array-shaped `item:[]` and `item:[""]` attempts
returned HTTP 500 and retained prior values. Cleanup verified zero test objects
and exact configuration/routes restoration. These are negative API observations,
not successful Terraform proof or all-tier-clear completeness.

Tagged [GatewayGroupItemField](https://github.com/opnsense/core/blob/26.7.7/src/opnsense/mvc/app/models/OPNsense/Routing/FieldTypes/GatewayGroupItemField.php)
only delegates scalar values when PHP `!empty(value)` is true. The base list
array setter filters empty members and delegates through that same override;
`GroupSettingsController` exposes only the generic model set action, not a
UUID-preserving first-tier clear action. No XML migration branch, private API or
manufactured nonempty sentinel is a supported provider workaround. The owner
must reject populated-to-empty first-tier updates before saving any other fields,
while allowing groups initially created with only later tiers populated and
updates that leave an already-empty first tier empty. Tier 2–5/description/pool
clears remain distinct from this explicitly upstream-gated operation.
