import '@testing-library/jest-dom'
import { vi } from 'vitest'

// ---------------------------------------------------------------------------
// Global Cesium mock
//
// Provides the minimum API surface so any test file that transitively imports
// 'cesium' does not crash in jsdom.  Per-file vi.mock('cesium', ...) calls in
// the 6 globe-related test files will automatically override this global mock
// for those files (Vitest's module-mock override semantics).
// ---------------------------------------------------------------------------
vi.mock('cesium', () => {
  class Cartesian3 {
    x: number
    y: number
    z: number
    constructor(x = 0, y = 0, z = 0) {
      this.x = x
      this.y = y
      this.z = z
    }
    static fromDegrees = vi.fn((lon: number, lat: number, alt = 0) => new Cartesian3(lon, lat, alt))
    static clone = vi.fn((c: { x: number; y: number; z: number }) => new Cartesian3(c.x, c.y, c.z))
    static lerp = vi.fn(
      (
        a: { x: number; y: number; z: number },
        b: { x: number; y: number; z: number },
        t: number,
        result: { x: number; y: number; z: number },
      ) => {
        result.x = a.x + (b.x - a.x) * t
        result.y = a.y + (b.y - a.y) * t
        result.z = a.z + (b.z - a.z) * t
        return result
      },
    )
    static UNIT_Z = new Cartesian3(0, 0, 1)
    static ZERO = new Cartesian3(0, 0, 0)
  }

  class Cartesian2 {
    x: number
    y: number
    constructor(x = 0, y = 0) {
      this.x = x
      this.y = y
    }
  }

  class Color {
    r: number
    g: number
    b: number
    alpha: number
    constructor(r = 1, g = 1, b = 1, a = 1) {
      this.r = r
      this.g = g
      this.b = b
      this.alpha = a
    }
    static fromCssColorString = vi.fn(() => new Color())
    static WHITE = new Color(1, 1, 1, 1)
    static BLACK = new Color(0, 0, 0, 1)
    static RED = new Color(1, 0, 0, 1)
    static GREEN = new Color(0, 1, 0, 1)
    static BLUE = new Color(0, 0, 1, 1)
    withAlpha = vi.fn(() => new Color())
  }

  class BillboardCollection {
    private _billboards: unknown[] = []
    add = vi.fn((opts: unknown) => {
      const billboard = { ...((opts as object) || {}), id: null }
      this._billboards.push(billboard)
      return billboard
    })
    remove = vi.fn()
    removeAll = vi.fn(() => {
      this._billboards = []
    })
    get length() {
      return this._billboards.length
    }
    get = vi.fn((i: number) => this._billboards[i])
    isDestroyed = vi.fn(() => false)
    destroy = vi.fn()
  }

  class LabelCollection {
    private _labels: unknown[] = []
    add = vi.fn((opts: unknown) => {
      const label = { ...((opts as object) || {}), id: null }
      this._labels.push(label)
      return label
    })
    remove = vi.fn()
    removeAll = vi.fn(() => {
      this._labels = []
    })
    get length() {
      return this._labels.length
    }
    get = vi.fn((i: number) => this._labels[i])
    isDestroyed = vi.fn(() => false)
    destroy = vi.fn()
  }

  class EllipsoidalOccluder {
    isPointVisible = vi.fn(() => true)
    isScaledSpacePointVisible = vi.fn(() => true)
    computeScaledSpacePosition = vi.fn((pos: unknown) => pos)
  }

  class Ellipsoid {
    static WGS84 = new Ellipsoid()
    scaleToGeodeticSurface = vi.fn((pos: unknown) => pos)
    cartographicToCartesian = vi.fn(() => new Cartesian3())
    cartesianToCartographic = vi.fn(() => ({ longitude: 0, latitude: 0, height: 0 }))
  }

  class UrlTemplateImageryProvider {
    constructor(_options?: unknown) {}
    ready = true
  }

  class Credit {
    constructor(public html = '') {}
  }

  class ConstantPositionProperty {
    constructor(public value?: unknown) {}
    getValue = vi.fn(() => this.value)
  }

  class HeadingPitchRange {
    heading: number
    pitch: number
    range: number
    constructor(heading = 0, pitch = 0, range = 0) {
      this.heading = heading
      this.pitch = pitch
      this.range = range
    }
  }

  class Matrix4 {
    static IDENTITY = new Matrix4()
    static clone = vi.fn((m: unknown) => m)
    static multiply = vi.fn((_a: unknown, _b: unknown, result: unknown) => result)
  }

  class Cesium3DTileset {
    constructor(_options?: unknown) {}
    ready = true
    show = true
  }

  class IonResource {
    static fromAssetId = vi.fn((_id: number) => Promise.resolve(`ion://asset/${_id}`))
  }

  class Ion {
    static defaultAccessToken = ''
  }

  class OpenStreetMapImageryProvider {
    constructor(_options?: unknown) {}
    ready = true
  }

  class ImageryLayer {
    constructor(_provider?: unknown, _options?: unknown) {}
    show = true
    alpha = 1
  }

  class Rectangle {
    west: number
    south: number
    east: number
    north: number
    constructor(west = 0, south = 0, east = 0, north = 0) {
      this.west = west
      this.south = south
      this.east = east
      this.north = north
    }
    static fromDegrees = vi.fn((west: number, south: number, east: number, north: number) => new Rectangle(west, south, east, north))
    static MAXIMUM_VALUE = new Rectangle(-Math.PI, -Math.PI / 2, Math.PI, Math.PI / 2)
  }

  class Cartographic {
    longitude: number
    latitude: number
    height: number
    constructor(longitude = 0, latitude = 0, height = 0) {
      this.longitude = longitude
      this.latitude = latitude
      this.height = height
    }
    static fromCartesian = vi.fn(
      (pos: { x: number; y: number; z?: number }) => ({
        longitude: (pos.x * Math.PI) / 180,
        latitude: (pos.y * Math.PI) / 180,
        height: pos.z ?? 0,
      }),
    )
    static fromDegrees = vi.fn((lon: number, lat: number, height = 0) => ({
      longitude: (lon * Math.PI) / 180,
      latitude: (lat * Math.PI) / 180,
      height,
    }))
  }

  const ScreenSpaceEventType = {
    LEFT_CLICK: 'LEFT_CLICK',
    LEFT_DOUBLE_CLICK: 'LEFT_DOUBLE_CLICK',
    MOUSE_MOVE: 'MOUSE_MOVE',
    RIGHT_CLICK: 'RIGHT_CLICK',
    WHEEL: 'WHEEL',
    MIDDLE_CLICK: 'MIDDLE_CLICK',
    LEFT_DOWN: 'LEFT_DOWN',
    LEFT_UP: 'LEFT_UP',
    PINCH_START: 'PINCH_START',
    PINCH_MOVE: 'PINCH_MOVE',
    PINCH_END: 'PINCH_END',
  } as const

  class ScreenSpaceEventHandler {
    setInputAction = vi.fn()
    removeInputAction = vi.fn()
    destroy = vi.fn()
    isDestroyed = vi.fn(() => false)
  }

  const SceneMode = {
    MORPHING: 0,
    COLUMBUS_VIEW: 1,
    SCENE2D: 2,
    SCENE3D: 3,
  } as const

  const LabelStyle = {
    FILL: 0,
    OUTLINE: 1,
    FILL_AND_OUTLINE: 2,
  } as const

  const VerticalOrigin = {
    CENTER: 0,
    BOTTOM: 1,
    BASELINE: 2,
    TOP: -1,
  } as const

  const HorizontalOrigin = {
    CENTER: 0,
    LEFT: 1,
    RIGHT: -1,
  } as const

  const CesiumMath = {
    toRadians: vi.fn((deg: number) => (deg * Math.PI) / 180),
    toDegrees: vi.fn((rad: number) => (rad * 180) / Math.PI),
    PI: Math.PI,
    TWO_PI: Math.PI * 2,
    PI_OVER_TWO: Math.PI / 2,
    PI_OVER_FOUR: Math.PI / 4,
    EPSILON1: 1e-1,
    EPSILON2: 1e-2,
    EPSILON6: 1e-6,
    EPSILON7: 1e-7,
    EPSILON10: 1e-10,
    EPSILON14: 1e-14,
    clamp: vi.fn((val: number, min: number, max: number) => Math.min(Math.max(val, min), max)),
  }

  return {
    Cartesian3,
    Cartesian2,
    Color,
    BillboardCollection,
    LabelCollection,
    EllipsoidalOccluder,
    Ellipsoid,
    UrlTemplateImageryProvider,
    Credit,
    ConstantPositionProperty,
    HeadingPitchRange,
    Matrix4,
    Cesium3DTileset,
    IonResource,
    Ion,
    Cartographic,
    ScreenSpaceEventType,
    ScreenSpaceEventHandler,
    SceneMode,
    LabelStyle,
    VerticalOrigin,
    HorizontalOrigin,
    OpenStreetMapImageryProvider,
    ImageryLayer,
    Rectangle,
    Math: CesiumMath,
  }
})

// Mock ResizeObserver
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

// Mock matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})

// MockIntersectionObserver
class MockIntersectionObserver {
  observe = vi.fn()
  disconnect = vi.fn()
  unobserve = vi.fn()
}

Object.defineProperty(window, 'IntersectionObserver', {
  writable: true,
  configurable: true,
  value: MockIntersectionObserver,
})

// Suppress console errors during tests (optional, remove if needed)
vi.spyOn(console, 'error').mockImplementation(() => {})
