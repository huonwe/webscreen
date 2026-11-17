# 设计思路

## 概览

多个user -> nexus -> 多个remote -> sdriver

单个user可以拥有多个remote
单个remote可以拥有多个user
一个remote只能和一个sdriver一一对应

一个remote的第一个user为master user
一个remote的sdriver配置由master user决定