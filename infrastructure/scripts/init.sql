-- Respondent Database Schema Initialization
-- This script sets up the initial database schema

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enable PostGIS extension (optional, for advanced geo queries)
-- CREATE EXTENSION IF NOT EXISTS "postgis";

-- Users table
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email TEXT UNIQUE NOT NULL,
  display_name TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Scenes table
CREATE TABLE IF NOT EXISTS scenes (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID REFERENCES users(id),
  name TEXT NOT NULL,
  camera JSONB NOT NULL,
  enabled_layers JSONB NOT NULL,
  filter_preset_id UUID,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Filter presets table
CREATE TABLE IF NOT EXISTS filter_presets (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name TEXT NOT NULL,
  style TEXT NOT NULL,
  params JSONB NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Data sources table
CREATE TABLE IF NOT EXISTS data_sources (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  config JSONB NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Entities table
CREATE TABLE IF NOT EXISTS entities (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  external_id TEXT,
  layer_type TEXT NOT NULL,
  name TEXT,
  metadata JSONB NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Observations table
CREATE TABLE IF NOT EXISTS observations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  entity_id UUID REFERENCES entities(id),
  ts TIMESTAMP NOT NULL,
  lat DOUBLE PRECISION NOT NULL,
  lon DOUBLE PRECISION NOT NULL,
  altitude_m DOUBLE PRECISION,
  velocity JSONB,
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for queries
CREATE INDEX IF NOT EXISTS idx_observations_entity_id ON observations(entity_id);
CREATE INDEX IF NOT EXISTS idx_observations_ts ON observations(ts DESC);
CREATE INDEX IF NOT EXISTS idx_entities_layer_type ON entities(layer_type);
CREATE INDEX IF NOT EXISTS idx_entities_external_id ON entities(external_id);

-- Camera feeds table
CREATE TABLE IF NOT EXISTS camera_feeds (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name TEXT NOT NULL,
  provider TEXT NOT NULL,
  lat DOUBLE PRECISION NOT NULL,
  lon DOUBLE PRECISION NOT NULL,
  stream_url TEXT NOT NULL,
  update_interval_seconds INT NOT NULL,
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Camera calibrations table
CREATE TABLE IF NOT EXISTS camera_calibrations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  camera_feed_id UUID REFERENCES camera_feeds(id),
  heading DOUBLE PRECISION,
  pitch DOUBLE PRECISION,
  roll DOUBLE PRECISION,
  fov DOUBLE PRECISION,
  range_m DOUBLE PRECISION,
  height_m DOUBLE PRECISION,
  offset_north_m DOUBLE PRECISION,
  offset_east_m DOUBLE PRECISION,
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Insert default filter presets
INSERT INTO filter_presets (name, style, params) VALUES
  ('Normal', 'NORMAL', '{"bloom": false, "sharpen": 0}'),
  ('CRT', 'CRT', '{"bloom": true, "sharpen": 10, "scanlines": true, "vignette": 0.3}'),
  ('Night Vision', 'NVG', '{"bloom": true, "sharpen": 20, "green_tint": true, "vignette": 0.8}'),
  ('FLIR', 'FLIR', '{"bloom": false, "sharpen": 30, "grayscale": true, "contrast": 1.5}')
ON CONFLICT DO NOTHING;

-- Insert default data sources (disabled by default)
INSERT INTO data_sources (name, type, config, enabled) VALUES
  ('OpenSky Flights', 'flights_commercial', '{"api_url": "https://opensky-network.org/api"}', false),
  ('ADSB Military', 'flights_military', '{"api_url": "https://api.adsb.lol"}', false),
  ('USGS Earthquakes', 'earthquakes', '{"api_url": "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/all_day.geojson"}', false),
  ('CelesTrak Satellites', 'satellites', '{"api_url": "https://celestrak.org/NORAD/elements/gp.php?GROUP=stations&FORMAT=tle"}', false),
  ('San Francisco CCTV', 'cctv', '{"city": "san-francisco"}', false)
ON CONFLICT DO NOTHING;
