/**
 * SmartValue — Renders text values with auto-detected clickable links,
 * HTML tag stripping, and optional truncation with "see more" / "see less" toggle.
 *
 * Generic component — not tied to any specific layer or entity type.
 */

import React, { useState, useMemo } from 'react';
import { Box, Typography } from '@mui/material';
import { FONT_FAMILY, DASHBOARD_TYPOGRAPHY } from '@respondent/core';

/** Matches http/https URLs in a string. */
const URL_REGEX = /https?:\/\/[^\s,)"'<>]+/g;

/** Quick check for HTML tags in a string. */
const HTML_TAG_REGEX = /<[a-z/][^>]*>/i;

/** Default character limit before truncation kicks in. */
const DEFAULT_TRUNCATE_AT = 120;

/** Strip all HTML tags and decode entities using the browser's DOMParser. */
function stripHtml(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html');
  return (doc.body.textContent ?? '').replace(/\s+/g, ' ').trim();
}

interface SmartValueProps {
  /** The raw text value to render. */
  value: string;
  /** Max characters before truncating. Set to 0 to disable. Default: 120. */
  truncateAt?: number;
  /** Base font size. Default: '0.8rem'. */
  fontSize?: string;
  /** Text color override. */
  color?: string;
  /** Font weight override. */
  fontWeight?: number;
  /** Text alignment. Default: 'left'. */
  textAlign?: 'left' | 'right';
  /** Use monospace font. Default: true. */
  mono?: boolean;
}

interface TextSegment {
  type: 'text' | 'url';
  content: string;
}

/** Split text into alternating text and URL segments. */
function parseSegments(text: string): TextSegment[] {
  const segments: TextSegment[] = [];
  let lastIndex = 0;

  for (const match of text.matchAll(URL_REGEX)) {
    const matchStart = match.index!;
    if (matchStart > lastIndex) {
      segments.push({ type: 'text', content: text.slice(lastIndex, matchStart) });
    }
    segments.push({ type: 'url', content: match[0] });
    lastIndex = matchStart + match[0].length;
  }

  if (lastIndex < text.length) {
    segments.push({ type: 'text', content: text.slice(lastIndex) });
  }

  return segments;
}

const SmartValue: React.FC<SmartValueProps> = ({
  value,
  truncateAt = DEFAULT_TRUNCATE_AT,
  fontSize = DASHBOARD_TYPOGRAPHY.dashboardBase.fontSize as string,
  color,
  fontWeight,
  textAlign = 'left',
  mono = true,
}) => {
  const [expanded, setExpanded] = useState(false);

  const cleanValue = useMemo(
    () => (HTML_TAG_REGEX.test(value) ? stripHtml(value) : value),
    [value],
  );

  const shouldTruncate = truncateAt > 0 && cleanValue.length > truncateAt;
  const displayText =
    shouldTruncate && !expanded ? cleanValue.slice(0, truncateAt) + '...' : cleanValue;

  const segments = useMemo(() => parseSegments(displayText), [displayText]);

  return (
    <Typography
      variant="caption"
      component="span"
      sx={{
        color: color ?? 'text.primary',
        fontSize,
        fontFamily: mono ? FONT_FAMILY : 'inherit',
        fontWeight: fontWeight ?? 500,
        textAlign,
        wordBreak: 'break-word',
        lineHeight: 1.5,
        display: 'inline',
      }}
    >
      {segments.map((seg, i) =>
        seg.type === 'url' ? (
          <Box
            key={i}
            component="a"
            href={seg.content}
            target="_blank"
            rel="noopener noreferrer"
            sx={{
              color: 'primary.main',
              textDecoration: 'none',
              wordBreak: 'break-all',
              '&:hover': {
                textDecoration: 'underline',
                opacity: 0.85,
              },
            }}
          >
            {seg.content}
          </Box>
        ) : (
          <React.Fragment key={i}>{seg.content}</React.Fragment>
        ),
      )}
      {shouldTruncate && (
        <Box
          component="span"
          data-testid="smart-value-toggle"
          data-expanded={expanded}
          onClick={(e: React.MouseEvent) => {
            e.stopPropagation();
            setExpanded(!expanded);
          }}
          sx={{
            color: 'primary.main',
            cursor: 'pointer',
            fontSize: DASHBOARD_TYPOGRAPHY.dashboardXs.fontSize,
            ml: 0.5,
            '&:hover': { textDecoration: 'underline' },
          }}
        >
          {expanded ? 'see less' : 'see more'}
        </Box>
      )}
    </Typography>
  );
};

export default SmartValue;
