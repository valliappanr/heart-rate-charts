FROM valliappanr/alpine-linux:1.2 AS build
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ENV REDIS_HOST=
RUN echo "I was built on a platform running on $BUILDPLATFORM, building this image for $TARGETPLATFORM" > /log

#RUN apk add --update usbip-utils
#RUN apk add alpine-sdk linux-lts-dev

#RUN modprobe usbip_core
#RUN modprobe usbip_host

RUN mkdir /app
COPY . /app
WORKDIR /app
RUN go build -o charts .
CMD /app/charts
