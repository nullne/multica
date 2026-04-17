const CLIP_PATH = `polygon(
  45% 62.1%, 45% 100%, 55% 100%, 55% 62.1%,
  81.8% 88.9%, 88.9% 81.8%, 62.1% 55%, 100% 55%,
  100% 45%, 62.1% 45%, 88.9% 18.2%, 81.8% 11.1%,
  55% 37.9%, 55% 0%, 45% 0%, 45% 37.9%,
  18.2% 11.1%, 11.1% 18.2%, 37.9% 45%, 0% 45%,
  0% 55%, 37.9% 55%, 11.1% 81.8%, 18.2% 88.9%
)`;

export function PwaIcon({ size }: { size: number }) {
  const glyphSize = Math.round(size * 0.42);

  return (
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background:
          "linear-gradient(160deg, rgb(17 24 39) 0%, rgb(31 41 55) 46%, rgb(59 130 246) 100%)",
        borderRadius: Math.round(size * 0.22),
      }}
    >
      <div
        style={{
          width: glyphSize,
          height: glyphSize,
          background: "white",
          clipPath: CLIP_PATH,
        }}
      />
    </div>
  );
}
