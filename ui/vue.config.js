const { defineConfig } = require('@vue/cli-service')

module.exports = defineConfig({
  publicPath: '/ui/',
  outputDir: 'dist',
  assetsDir: '',
  productionSourceMap: false,
  transpileDependencies: true,
  configureWebpack: {
    optimization: {
      splitChunks: {
        chunks: 'all',
        minSize: 20000,
        maxSize: 250000,
      }
    }
  },
  devServer: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
}) 