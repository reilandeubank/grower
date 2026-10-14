import styles from "./Logo.module.css";

export interface LogoProps {
  /** Mark size in px. */
  size?: number;
  /** Show the lowercase "grower" wordmark next to or under the mark. */
  wordmark?: boolean;
  layout?: "inline" | "stacked";
  className?: string;
}

/** The leaf mark from assets/brand/leaf.svg, painted with tokens so it follows the theme. */
export function Logo({ size = 40, wordmark = false, layout = "inline", className }: LogoProps) {
  const big = size >= 56;
  return (
    <span className={[styles.logo, styles[layout], className].filter(Boolean).join(" ")}>
      <svg
        className={styles.mark}
        width={size}
        height={size}
        viewBox="0 0 512 512"
        role={wordmark ? undefined : "img"}
        aria-hidden={wordmark ? true : undefined}
        aria-label={wordmark ? undefined : "grower"}
      >
        <rect width="512" height="512" rx="112" className={styles.tile} />
        <g transform="translate(256 256) scale(0.8) translate(-256 -256)">
          <path d="M136 376 A240 240 0 0 1 376 136 A240 240 0 0 1 136 376 Z" className={styles.leaf} />
          <path d="M104 408 L330 182" className={styles.stem} strokeWidth="16" strokeLinecap="round" />
          <path d="M104 408 L136 376" className={styles.stemTail} strokeWidth="16" strokeLinecap="round" />
        </g>
      </svg>
      {wordmark && <span className={`${big ? "t-display" : "t-title"} ${styles.word}`}>grower</span>}
    </span>
  );
}
