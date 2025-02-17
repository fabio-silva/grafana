import resolve from '@rollup/plugin-node-resolve';
import path from 'path';
import dts from 'rollup-plugin-dts';
import esbuild from 'rollup-plugin-esbuild';
import { externals } from 'rollup-plugin-node-externals';
import svg from 'rollup-plugin-svg-import';

import pkg from './package.json' assert { type: 'json' };

export default [
  {
    input: 'src/index.ts',
    plugins: [externals({ deps: true, packagePath: './package.json' }), resolve(), svg({ stringify: true }), esbuild()],
    output: [
      {
        format: 'cjs',
        sourcemap: true,
        dir: path.dirname(pkg.publishConfig.main),
      },
      {
        format: 'esm',
        sourcemap: true,
        dir: path.dirname(pkg.publishConfig.module),
        preserveModules: true,
        preserveModulesRoot: path.join(process.env.PROJECT_CWD, 'packages/grafana-ui/src'),
      },
    ],
  },
  {
    input: './compiled/index.d.ts',
    plugins: [dts()],
    output: {
      file: pkg.publishConfig.types,
      format: 'es',
    },
  },
];
