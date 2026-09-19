import { renderHook } from '@testing-library/react';
import { describe, it, expect, vi, afterEach, type MockInstance } from 'vitest';
import { ThemeProvider, createTheme } from '@mui/material/styles';
import React from 'react';
import { useResponsive } from '@respondent/core';

const theme = createTheme({
  breakpoints: {
    values: { xs: 0, sm: 600, md: 900, lg: 1200, xl: 1536 },
  },
});

function wrapper({ children }: { children: React.ReactNode }) {
  return React.createElement(ThemeProvider, { theme }, children);
}

describe('useResponsive', () => {
  let matchMediaSpy: MockInstance<(query: string) => MediaQueryList>;

  afterEach(() => {
    matchMediaSpy?.mockRestore();
  });

  function mockViewport(width: number) {
    matchMediaSpy = vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => {
      // Parse MUI breakpoint media queries
      const minMatch = query.match(/\(min-width:(\d+)px\)/);
      const maxMatch = query.match(/\(max-width:(\d+(?:\.\d+)?)px\)/);

      let matches = true;

      if (minMatch) {
        matches = matches && width >= parseInt(minMatch[1]);
      }
      if (maxMatch) {
        matches = matches && width <= parseFloat(maxMatch[1]);
      }

      return {
        matches,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      } as unknown as MediaQueryList;
    });
  }

  it('returns isMobile=true for small viewports', () => {
    mockViewport(440);
    const { result } = renderHook(() => useResponsive(), { wrapper });
    expect(result.current.isMobile).toBe(true);
    expect(result.current.isDesktop).toBe(false);
  });

  it('returns isDesktop=true for large viewports', () => {
    mockViewport(1400);
    const { result } = renderHook(() => useResponsive(), { wrapper });
    expect(result.current.isMobile).toBe(false);
    expect(result.current.isDesktop).toBe(true);
  });

  it('returns isTablet=true for medium viewports', () => {
    mockViewport(1000);
    const { result } = renderHook(() => useResponsive(), { wrapper });
    expect(result.current.isTablet).toBe(true);
    expect(result.current.isMobile).toBe(false);
    expect(result.current.isDesktop).toBe(false);
  });
});
