/** @type {import('tailwindcss').Config} */
const colors = require("tailwindcss/colors");

module.exports = {
  content: ["./src/*.{html,js,css}", "./templates/**/*.templ"],
  theme: {
    colors: colors,
    extend: {
      colors: {
        primary: {
          dark: "#A38B3E",
          light: "#F5BC00",
        },
      },
    },
  },
  plugins: [],
};
