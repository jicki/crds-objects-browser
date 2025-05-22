const { defineConfig } = require('@vue/cli-service')

module.exports = defineConfig({
  publicPath: '/ui/',
  outputDir: 'dist',
  assetsDir: 'assets',
  productionSourceMap: false,
  transpileDependencies: true,
  configureWebpack: {
    performance: {
      hints: false,
      maxEntrypointSize: 512000,
      maxAssetSize: 512000
    },
    optimization: {
      splitChunks: {
        cacheGroups: {
          defaultVendors: {
            name: 'chunk-vendors',
            test: /[\\/]node_modules[\\/]/,
            priority: -10,
            chunks: 'initial',
            reuseExistingChunk: true
          },
          common: {
            name: 'chunk-common',
            minChunks: 2,
            priority: -20,
            chunks: 'initial',
            reuseExistingChunk: true
          }
        }
      }
    }
  },
  css: {
    extract: {
      ignoreOrder: true
    },
    loaderOptions: {
      css: {
        // 启用 CSS Modules
        modules: {
          auto: true,
          localIdentName: '[name]_[local]_[hash:base64:5]'
        }
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