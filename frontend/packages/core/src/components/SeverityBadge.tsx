import { Box, type SxProps } from '@mui/material';
import { theme, alpha, semanticColors } from '../theme';

const SEVERITY_COLORS: Record<string, { bg: string; text: string }> = {
  critical: { bg: 'rgba(255,80,80,0.15)', text: '#ff5555' },
  high: { bg: 'rgba(255,165,0,0.12)', text: '#ffaa00' },
  medium: { bg: semanticColors.primary.alpha10, text: theme.palette.primary.main },
  low: {
    bg: alpha(theme.palette.primary.main, 0.06),
    text: alpha(theme.palette.primary.main, 0.5),
  },
  info: { bg: 'rgba(0,170,255,0.1)', text: '#00aaff' },
};

interface SeverityBadgeProps {
  attention: string;
  sx?: SxProps;
}

export function SeverityBadge({ attention, sx }: SeverityBadgeProps) {
  const colors = SEVERITY_COLORS[attention] ?? SEVERITY_COLORS.info;
  return (
    <Box
      component="span"
      sx={{
        fontSize: '0.65rem',
        fontWeight: 700,
        px: 0.6,
        py: 0.15,
        borderRadius: 0.5,
        bgcolor: colors.bg,
        color: colors.text,
        fontFamily: 'monospace',
        letterSpacing: '0.5px',
        ...sx,
      }}
    >
      {attention.toUpperCase()}
    </Box>
  );
}
