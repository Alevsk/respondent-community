/**
 * OverviewTab — AI Analysis section tests.
 *
 * The enrichment worker stores an operation's result nested under the operation
 * name when the source has no output_mapping (the shape used by every AI source
 * today), and at the top level when it does. These tests pin both shapes plus
 * the empty/absent cases so the AI Analysis section renders its content and
 * never shows an empty header.
 */

import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import CssBaseline from '@mui/material/CssBaseline';
import type { EntityDetailResponse } from '@respondent/core';
import OverviewTab from './OverviewTab';
import { MediaProvider } from '@/features/media/MediaProvider';

const theme = createTheme({
  palette: {
    mode: 'dark',
    primary: { main: '#00ff9d' },
    secondary: { main: '#ff4d4d' },
    background: { default: '#000000', paper: '#0a0a0a' },
    text: { primary: '#ffffff', secondary: '#888888' },
  },
});

const renderTab = (aiMetadataJson?: string) => {
  const detail: EntityDetailResponse = {
    entity: {
      id: 'news_articles:test',
      externalId: 'test',
      layerType: 'news_articles',
      name: 'Test Article',
      metadata: {},
      aiMetadataJson,
    },
  };
  return render(
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <MediaProvider>
        <OverviewTab
          entityId="news_articles:test"
          layerType="news_articles"
          detail={detail}
          isLoading={false}
        />
      </MediaProvider>
    </ThemeProvider>,
  );
};

describe('OverviewTab — AI Analysis section', () => {
  it('renders enrichment nested under the operation name (the regression case)', () => {
    renderTab(
      JSON.stringify({
        news_geo_enrichment: {
          severity: 'high',
          summary: 'A serious humanitarian crisis is unfolding in the region.',
          category: 'humanitarian_crisis',
          country: 'Haiti',
          key_actors: ['World Food Programme', 'UN'],
          affected_population: 'large',
        },
      }),
    );

    expect(screen.getByText('AI Analysis')).toBeInTheDocument();
    expect(screen.getByText('high')).toBeInTheDocument();
    expect(screen.getByText(/serious humanitarian crisis/i)).toBeInTheDocument();
    expect(screen.getByText('Category')).toBeInTheDocument();
    expect(screen.getByText('humanitarian_crisis')).toBeInTheDocument();
    expect(screen.getByText('Country')).toBeInTheDocument();
    expect(screen.getByText('Haiti')).toBeInTheDocument();
    expect(screen.getByText('Key actors')).toBeInTheDocument();
    expect(screen.getByText('World Food Programme, UN')).toBeInTheDocument();
    expect(screen.getByText('large')).toBeInTheDocument();
  });

  it('skips position keys already shown in the Location section', () => {
    renderTab(
      JSON.stringify({
        news_geo_enrichment: {
          severity: 'low',
          summary: 'Routine update.',
          lat: 18.5944,
          lon: -72.3074,
          location_confidence: 0.9,
          country_iso3: 'HTI',
        },
      }),
    );

    expect(screen.getByText('AI Analysis')).toBeInTheDocument();
    expect(screen.queryByText('HTI')).not.toBeInTheDocument();
    expect(screen.queryByText('Country iso3')).not.toBeInTheDocument();
    expect(screen.queryByText('Location confidence')).not.toBeInTheDocument();
    expect(screen.queryByText('18.5944')).not.toBeInTheDocument();
  });

  it('renders flat (output_mapping) enrichment at the top level', () => {
    renderTab(
      JSON.stringify({
        severity: 'moderate',
        summary: 'Mapped output without nesting.',
        threat_level: 'minimal',
      }),
    );

    expect(screen.getByText('AI Analysis')).toBeInTheDocument();
    expect(screen.getByText('moderate')).toBeInTheDocument();
    expect(screen.getByText(/Mapped output without nesting/i)).toBeInTheDocument();
    expect(screen.getByText('Threat level')).toBeInTheDocument();
    expect(screen.getByText('minimal')).toBeInTheDocument();
  });

  it('merges fields from multiple operations', () => {
    renderTab(
      JSON.stringify({
        op_one: { severity: 'critical', summary: 'First operation summary.' },
        op_two: { confidence: 'high', note: 'Second operation note.' },
      }),
    );

    expect(screen.getByText('AI Analysis')).toBeInTheDocument();
    expect(screen.getByText('critical')).toBeInTheDocument();
    expect(screen.getByText(/First operation summary/i)).toBeInTheDocument();
    expect(screen.getByText('Confidence')).toBeInTheDocument();
    expect(screen.getByText('Note')).toBeInTheDocument();
    expect(screen.getByText(/Second operation note/i)).toBeInTheDocument();
  });

  it('resolves colliding keys deterministically — first operation wins', () => {
    renderTab(
      JSON.stringify({
        op_one: { severity: 'critical', summary: 'First wins.' },
        op_two: { severity: 'low', summary: 'Second is dropped.' },
      }),
    );

    expect(screen.getByText('critical')).toBeInTheDocument();
    expect(screen.queryByText('low')).not.toBeInTheDocument();
    expect(screen.getByText(/First wins/i)).toBeInTheDocument();
    expect(screen.queryByText(/Second is dropped/i)).not.toBeInTheDocument();
  });

  it('lets explicit top-level fields win over nested operation fields', () => {
    renderTab(
      JSON.stringify({
        severity: 'critical',
        news_geo_enrichment: { severity: 'low', summary: 'Nested summary.' },
      }),
    );

    expect(screen.getByText('critical')).toBeInTheDocument();
    expect(screen.queryByText('low')).not.toBeInTheDocument();
    expect(screen.getByText(/Nested summary/i)).toBeInTheDocument();
  });

  it('renders the section when only a severity is present (no summary or fields)', () => {
    renderTab(JSON.stringify({ news_geo_enrichment: { severity: 'high' } }));
    expect(screen.getByText('AI Analysis')).toBeInTheDocument();
    expect(screen.getByText('high')).toBeInTheDocument();
  });

  it('renders the section when only non-severity fields are present', () => {
    renderTab(JSON.stringify({ news_geo_enrichment: { category: 'cyber_attack' } }));
    expect(screen.getByText('AI Analysis')).toBeInTheDocument();
    expect(screen.getByText('Category')).toBeInTheDocument();
    expect(screen.getByText('cyber_attack')).toBeInTheDocument();
  });

  it('omits the section when only position skip-keys remain', () => {
    renderTab(
      JSON.stringify({
        news_geo_enrichment: {
          lat: 18.5944,
          lon: -72.3074,
          location_confidence: 0.9,
          country_iso3: 'HTI',
        },
      }),
    );
    expect(screen.queryByText('AI Analysis')).not.toBeInTheDocument();
  });

  it('omits the section when the enrichment result is empty', () => {
    renderTab(JSON.stringify({ news_geo_enrichment: {} }));
    expect(screen.queryByText('AI Analysis')).not.toBeInTheDocument();
  });

  it('omits the section when no AI metadata is present', () => {
    renderTab(undefined);
    expect(screen.queryByText('AI Analysis')).not.toBeInTheDocument();
  });

  it('omits the section when AI metadata is invalid JSON', () => {
    renderTab('{not valid json');
    expect(screen.queryByText('AI Analysis')).not.toBeInTheDocument();
  });
});
