---
title: "Icon Shapes"
description: "All 56 available icon shapes for entity display"
weight: 4
---

Respondent provides 56 icon shapes for entity rendering on the 3D globe. Set the shape in your source definition's `display.icon.shape` field. Unknown shape names fall back to `dot`.

```yaml
display:
  icon:
    shape: flight
    rotatable: true
    scale: 1.0
```

{{< callout type="tip" title="Directional icons" >}}
Enable `rotatable: true` for directional entities like aircraft and ships. The icon rotates to match the entity's heading (from `observation.velocity.heading`).
{{< /callout >}}

## Aviation

| Shape | Name | Notes |
|-------|------|-------|
| flight | `flight` | Standard aircraft icon. Use with `rotatable: true`. |
| flight-alt-a | `flight-alt-a` | Alternative aircraft shape A |
| flight-alt-b | `flight-alt-b` | Alternative aircraft shape B |
| flight-alt-c | `flight-alt-c` | Alternative aircraft shape C |
| drone | `drone` | Unmanned aerial vehicle |
| helicopter | `helicopter` | Rotary-wing aircraft |

## Space

| Shape | Name | Notes |
|-------|------|-------|
| diamond | `diamond` | Diamond shape, also used for satellites |
| satellite | `satellite` | Alias for `diamond` |
| iss | `iss` | International Space Station |
| rocket | `rocket` | Rocket or launch vehicle |
| telescope | `telescope` | Space telescope or observatory |
| meteor | `meteor` | Meteor or fireball |

## Maritime

| Shape | Name | Notes |
|-------|------|-------|
| ship | `ship` | Vessel icon. Use with `rotatable: true`. |
| anchor | `anchor` | Port or anchored vessel |
| wave | `wave` | Wave or ocean event |

## Military

| Shape | Name | Notes |
|-------|------|-------|
| missile | `missile` | Missile or projectile |
| tank | `tank` | Ground vehicle |
| shield | `shield` | Defensive position or installation |
| crosshair | `crosshair` | Target or point of interest |
| radar | `radar` | Radar installation or coverage |
| explosion | `explosion` | Explosion or impact event |
| bullseye | `bullseye` | Target or impact zone |

## Weather

| Shape | Name | Notes |
|-------|------|-------|
| lightning | `lightning` | Lightning strike or thunderstorm |
| cloud | `cloud` | Cloud or weather event |
| wind | `wind` | Wind event or station |
| tornado | `tornado` | Tornado or severe wind event |
| snowflake | `snowflake` | Snow or winter weather |
| hurricane | `hurricane` | Hurricane or tropical cyclone |
| flood | `flood` | Flood event |

## Hazard

| Shape | Name | Notes |
|-------|------|-------|
| warning | `warning` | General warning or alert |
| radiation | `radiation` | Radiation hazard |
| nuclear | `nuclear` | Nuclear event or facility |
| biohazard | `biohazard` | Biological hazard |
| skull | `skull` | Danger or fatality |
| fire | `fire` | Active fire |
| volcano | `volcano` | Volcanic activity |

## Seismic

| Shape | Name | Notes |
|-------|------|-------|
| ripple | `ripple` | Seismic wave or earthquake |
| crack | `crack` | Fault line or ground rupture |

## Radio

| Shape | Name | Notes |
|-------|------|-------|
| radio | `radio` | Radio signal or communication |

## Infrastructure

| Shape | Name | Notes |
|-------|------|-------|
| tower | `tower` | Communication tower or antenna |
| building | `building` | Building or structure |
| bridge | `bridge` | Bridge or crossing |
| factory | `factory` | Industrial facility |
| powerplant | `powerplant` | Power generation facility |
| crane | `crane` | Construction crane |
| warehouse | `warehouse` | Storage or logistics facility |

## Geometry

| Shape | Name | Notes |
|-------|------|-------|
| dot | `dot` | Default shape. Simple point. |
| star | `star` | Star shape |
| hexagon | `hexagon` | Hexagonal shape |
| circle-ring | `circle-ring` | Ring or circular outline |
| chevron | `chevron` | Chevron or arrow shape |
| triangle | `triangle` | Triangle shape |
| pentagon | `pentagon` | Pentagon shape |
| cross | `cross` | Cross or plus shape |

## Other

| Shape | Name | Notes |
|-------|------|-------|
| marker | `marker` | Map marker or pin |
| tree | `tree` | Tree or vegetation |
