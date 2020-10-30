make build-in-docker
docker build -t swr.cn-north-4.myhuaweicloud.com/hwstaff_f00341949/ecs_as_test:test .
docker login -u cn-north-4@S3AU70WWBLD25HFJ7LNC -p 8439772dfb949e2725c7aea979216fe7050c2727ab97086f9e07c1537251b7f5 swr.cn-north-4.myhuaweicloud.com
docker push swr.cn-north-4.myhuaweicloud.com/hwstaff_f00341949/ecs_as_test:test
