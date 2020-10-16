#/bin/bash

make build-in-docker
docker login -u cn-north-4@8QB8WTT3E5XX5FA4LCUR -p c8dfd56336bcbe5b8e1e763b6b97e2759263b86d7aac87712d4b19c0c04286d2 swr.cn-north-4.myhuaweicloud.com
docker build -t swr.cn-north-4.myhuaweicloud.com/hwstaff_f00341949/shiqi:test .
docker push swr.cn-north-4.myhuaweicloud.com/hwstaff_f00341949/shiqi:test
