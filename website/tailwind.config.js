module.exports = {
  purge: {
    mode: "layers",
    content: [
      "./src/**/*.vue",
      "./src/**/*.js",
      "./src/**/*.jsx",
      "./src/**/*.html",
      "./src/**/*.pug",
      "./src/**/*.md",
    ],
    safelistPatternsChildren: [/token$/],
    layers: ["components", "utilities"],
  },
  theme: {
    extend: {
      colors: {
        ui: {
          background: "var(--color-ui-background)",
          sidebar: "var(--color-ui-sidebar)",
          typo: "var(--color-ui-typo)",
          primary: "var(--color-ui-primary)",
          border: "var(--color-ui-border)",
        },
        "primary-color": {
          100: "#EBF7F9",
          200: "#CCEBF1",
          300: "#ADDFE8",
          400: "#70C7D6",
          500: "#32AFC5",
          600: "#2D9EB1",
          700: "#1E6976",
          800: "#174F59",
          900: "#0F353B",
        },
        "logo-teal": {
          default: "#32AFC5",
          darker: "#2c95a8",
        },
        "logo-colors": {
          1: "#32AFC5",
          2: "#6D9CBC",
          3: "#AC80A3",
          4: "#E66B8D",
        },
      },
      spacing: {
        sm: "24rem",
      },
      screens: {
        xxl: "1400px",
      },
    },
    container: {
      center: true,
      padding: "1rem",
      screens: {
        sm: "100%",
        md: "100%",
        lg: "1600px",
        xl: "1750px",
      },
    },
  },
  variants: {},
  plugins: [
    require("tailwindcss-grid")({
      grids: [2, 3, 4, 5, 6, 8, 10, 12],
      gaps: {
        0: "0",
        4: "1rem",
        8: "2rem",
        12: "3rem",
        "4-x": "1rem",
        "4-y": "1rem",
      },
      autoMinWidths: {
        "16": "4rem",
        "24": "6rem",
        "300px": "300px",
      },
      variants: ["responsive"],
    }),
  ],
};
